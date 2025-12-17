package api

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"aexon/internal/events"
	"aexon/internal/monitor"
	"aexon/internal/provider/lxc"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// PERMITIR TUDO (CORS FIX):
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// StreamTelemetry lida com a conexão WebSocket.
func StreamTelemetry(c *gin.Context, instanceService *lxc.InstanceService, metricProcessor func([]lxc.InstanceMetric)) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Falha no upgrade WebSocket: %v", err)
		return
	}
	defer conn.Close()

	// Contexto para controlar o ciclo de vida das goroutines auxiliares
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Garante que o poller pare ao sair desta função

	sendChan := make(chan interface{}, 256)

	// A. Registra cliente para receber eventos globais
	registerClient(conn, sendChan)
	defer unregisterClient(conn)

	// B. Poller de Métricas
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return // Encerra a goroutine quando o cliente desconecta
			case <-ticker.C:
				metrics, err := instanceService.ListInstances()
				if err != nil {
					log.Printf("Erro ao coletar métricas: %v", err)
					continue
				}

				// Process metrics
				if metricProcessor != nil {
					metricProcessor(metrics)
				}

				// Tenta enviar. Se o contexto for cancelado ou buffer cheio, desiste.
				select {
				case sendChan <- gin.H{"type": "instance_metrics", "data": metrics}:
				case <-ctx.Done():
					return
				default:
					// Buffer cheio, drop metric frame
				}

				// Get and send host stats
				hostStats, err := monitor.GetHostStats()
				if err != nil {
					log.Printf("Erro ao coletar estatísticas do host: %v", err)
				} else {
					hostTelemetry := map[string]interface{}{
						"type": "host_telemetry",
						"data": hostStats,
					}

					select {
					case sendChan <- hostTelemetry:
					case <-ctx.Done():
						return
					default:
						// Buffer cheio, drop host telemetry frame
					}
				}
			}
		}
	}()

	// C. Loop de Escrita (Único dono do conn.WriteJSON)
	// Este loop roda na goroutine principal do handler.
	// Quando ele sai (erro de escrita ou sendChan fechado), o defer cancel() mata o poller.
	for {
		select {
		case payload, ok := <-sendChan:
			if !ok {
				return // Canal fechado
			}
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteJSON(payload); err != nil {
				log.Printf("Erro de escrita no WebSocket: %v", err)
				return // Sai da função, aciona defers
			}
		case <-c.Request.Context().Done(): // Cliente fechou conexão TCP abruptamente
			return
		}
	}
}

// --- Gerenciamento Global de Canais de Clientes ---

var clientMap = make(map[*websocket.Conn]chan interface{})
var mapLock sync.Mutex

func registerClient(conn *websocket.Conn, ch chan interface{}) {
	mapLock.Lock()
	clientMap[conn] = ch
	mapLock.Unlock()
}

func unregisterClient(conn *websocket.Conn) {
	mapLock.Lock()
	if _, ok := clientMap[conn]; ok {
		delete(clientMap, conn)
	}
	mapLock.Unlock()
}

// InitBroadcaster deve ser chamado no main.
func InitBroadcaster() {
	go func() {
		for evt := range events.GlobalBus {
			mapLock.Lock()
			for _, ch := range clientMap {
				// Non-blocking send
				select {
				case ch <- evt:
				default:
					// Cliente lento, dropando evento
				}
			}
			mapLock.Unlock()
		}
	}()
}
