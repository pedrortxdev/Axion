package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"aexon/internal/auth"
	"aexon/internal/provider/lxc"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type wsWriter struct {
	conn *websocket.Conn
}

func (w *wsWriter) Write(p []byte) (n int, err error) {
	err = w.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *wsWriter) Close() error {
	return nil
}

// TerminalHandler gerencia a sessão de terminal interativo via WebSocket.
func TerminalHandler(c *gin.Context, instanceService *lxc.InstanceService) {
	name := c.Param("name")
	token := c.Query("token")

	if token == "" {
		c.AbortWithStatusJSON(401, gin.H{"error": "Token missing"})
		return
	}
	if _, err := auth.ValidateToken(token); err != nil {
		c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Terminal Upgrade Failed: %v", err)
		return
	}
	defer conn.Close()

	stdinReader, stdinWriter := io.Pipe()
	stdoutWriter := &wsWriter{conn: conn}

	// Controle: agora usando *websocket.Conn
	controlCh := make(chan *websocket.Conn)

	go func() {
		controlFunc := func(control *websocket.Conn) {
			controlCh <- control
		}

		err := instanceService.ExecInteractive(
			name,
			[]string{"/bin/bash"},
			stdinReader,
			stdoutWriter,
			stdoutWriter,
			controlFunc,
		)

		if err != nil {
			log.Printf("Terminal session ended with error: %v", err)
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nSession ended: %v\r\n", err)))
		}
		conn.Close()
	}()

	var execControl *websocket.Conn
	select {
	case execControl = <-controlCh:
		// OK
	case <-time.After(10 * time.Second):
		log.Printf("Timeout waiting for exec control")
		return
	}
	// O execControl é um WS que pode receber JSON de resize/signal.
	// Podemos fechar quando terminarmos.
	defer execControl.Close()

	for {
		mt, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if mt == websocket.TextMessage {
			var msg struct {
				Type string `json:"type"`
				Cols int    `json:"cols"`
				Rows int    `json:"rows"`
			}

			// Se for resize, enviamos para o canal de controle do LXD
			// O protocolo LXD espera JSON: { "command": "window-resize", "width": ..., "height": ... }
			if err := json.Unmarshal(message, &msg); err == nil && msg.Type == "resize" {
				if execControl != nil {
					lxdMsg := map[string]interface{}{
						"command": "window-resize",
						"width":   msg.Cols, // Cols = Width
						"height":  msg.Rows, // Rows = Height
					}
					execControl.WriteJSON(lxdMsg)
				}
				continue
			}

			stdinWriter.Write(message)
		} else {
			stdinWriter.Write(message)
		}
	}
}
