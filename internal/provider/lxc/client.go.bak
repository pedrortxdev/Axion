package lxc

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"aexon/internal/utils"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"github.com/gorilla/websocket"
)

// InstanceService gerencia a comunicação com o daemon LXD.
type InstanceService struct {
	server lxd.InstanceServer
	locks  sync.Map
}

type InstanceMetric struct {
	Name             string                       `json:"name"`
	Status           string                       `json:"status"`
	MemoryUsageBytes int64                        `json:"memory_usage_bytes"`
	CPUUsageSeconds  int64                        `json:"cpu_usage_seconds"`
	Config           map[string]string            `json:"config"`
	Devices          map[string]map[string]string `json:"devices"`
}

const (
	MaxGlobalCPU   = 8
	MaxGlobalRAMMB = 8192
)

type FileEntry struct {
	Name string `json:"name"`
	Type string `json:"type"` // "file" or "directory"
}

func NewClient() (*InstanceService, error) {
	const socketPath = "/var/snap/lxd/common/lxd/unix.socket"

	if info, err := os.Stat(socketPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("socket LXD não encontrado em %s", socketPath)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permissão negada ao acessar %s", socketPath)
		}
		return nil, fmt.Errorf("erro ao verificar socket LXD: %w", err)
	} else {
		if info.Mode().Perm()&0600 == 0 {
		}
	}

	c, err := lxd.ConnectLXDUnix(socketPath, nil)
	if err != nil {
		if os.IsPermission(err) {
			return nil, fmt.Errorf("erro de permissão na conexão LXD: %w", err)
		}
		return nil, fmt.Errorf("falha ao conectar no socket Unix do LXD: %w", err)
	}

	return &InstanceService{
		server: c,
	}, nil
}

// CheckPortAvailability verifica se a porta do host está livre e dentro do range permitido.
func (s *InstanceService) CheckPortAvailability(hostPort int) error {
	if hostPort < 10000 || hostPort > 60000 {
		return fmt.Errorf("porta %d inválida. Permitido apenas entre 10000 e 60000", hostPort)
	}

	instances, err := s.server.GetInstancesFull(api.InstanceTypeContainer)
	if err != nil {
		return fmt.Errorf("falha ao verificar uso de portas: %w", err)
	}

	targetListen := fmt.Sprintf("0.0.0.0:%d", hostPort)

	for _, inst := range instances {
		for _, dev := range inst.Devices {
			if dev["type"] == "proxy" {
				if strings.Contains(dev["listen"], targetListen) {
					return fmt.Errorf("porta %d já está em uso pelo container '%s'", hostPort, inst.Name)
				}
			}
		}
	}

	return nil
}

// ExecInteractive inicia uma sessão interativa (shell) no container.
func (s *InstanceService) ExecInteractive(name string, cmd []string, stdin io.ReadCloser, stdout io.WriteCloser, stderr io.WriteCloser, controlHandler func(*websocket.Conn)) error {
	req := api.InstanceExecPost{
		Command:     cmd,
		WaitForWS:   true,
		Interactive: true,
		Environment: map[string]string{
			"TERM": "xterm-256color",
			"HOME": "/root",
		},
	}

	args := lxd.InstanceExecArgs{
		Stdin:    stdin,
		Stdout:   stdout,
		Stderr:   stderr,
		Control:  controlHandler,
	}

	log.Printf("[LXD Provider] Iniciando terminal interativo em '%s'", name)
	op, err := s.server.ExecInstance(name, req, &args)
	if err != nil {
		return fmt.Errorf("falha ao iniciar execução interativa: %w", err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("erro durante execução interativa: %w", err)
	}

	return nil
}

func (s *InstanceService) CheckGlobalQuota(requestCPU int, requestRAMMB int64) error {
	instances, err := s.server.GetInstancesFull(api.InstanceTypeContainer)
	if err != nil {
		return fmt.Errorf("falha ao auditar uso de recursos: %w", err)
	}

	var totalCPU int
	var totalRAMMB int64

	for _, inst := range instances {
		cpuStr := inst.Config["limits.cpu"]
		memStr := inst.Config["limits.memory"]

		cpu := utils.ParseCpuCores(cpuStr)
		if cpu == 0 {
			cpu = 1
		}

		ram := utils.ParseMemoryToMB(memStr)
		if ram == 0 {
			ram = 1024
		}

		totalCPU += cpu
		totalRAMMB += ram
	}

	log.Printf("[Quota Governance] Uso Atual: %d/%d CPU, %d/%d MB RAM. Solicitado: +%d CPU, +%d MB RAM",
		totalCPU, MaxGlobalCPU, totalRAMMB, MaxGlobalRAMMB, requestCPU, requestRAMMB)

	if totalCPU+requestCPU > MaxGlobalCPU {
		return fmt.Errorf("QUOTA EXCEEDED: CPU insuficiente. Disponível: %d, Solicitado: %d", MaxGlobalCPU-totalCPU, requestCPU)
	}

	if totalRAMMB+requestRAMMB > MaxGlobalRAMMB {
		return fmt.Errorf("QUOTA EXCEEDED: RAM insuficiente. Disponível: %d MB, Solicitado: %d MB", MaxGlobalRAMMB-totalRAMMB, requestRAMMB)
	}

	return nil
}

func (s *InstanceService) ListInstances() ([]InstanceMetric, error) {
	instances, err := s.server.GetInstancesFull(api.InstanceTypeContainer)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar instâncias: %w", err)
	}

	var metrics []InstanceMetric

	for _, inst := range instances {
		state, _, err := s.server.GetInstanceState(inst.Name)

		cpuSeconds := int64(0)
		memBytes := int64(0)

		if err != nil {
			log.Printf("Aviso: não foi possível obter estado para %s: %v", inst.Name, err)
		} else {
			cpuSeconds = state.CPU.Usage / 1_000_000_000
			memBytes = state.Memory.Usage
		}

		metrics = append(metrics, InstanceMetric{
			Name:             inst.Name,
			Status:           inst.Status,
			MemoryUsageBytes: memBytes,
			CPUUsageSeconds:  cpuSeconds,
			Config:           inst.Config,
			Devices:          inst.Devices,
		})
	}

	return metrics, nil
}

func (s *InstanceService) UpdateInstanceState(name string, action string) error {
	if _, busy := s.locks.LoadOrStore(name, true); busy {
		return fmt.Errorf("LOCKED: container '%s' já está processando um comando. Tente novamente em alguns segundos", name)
	}
	defer s.locks.Delete(name)

	req := api.InstanceStatePut{
		Action:  action,
		Timeout: -1,
	}

	log.Printf("[LXD Provider] Iniciando ação '%s' em '%s' (Lock adquirido)", action, name)

	op, err := s.server.UpdateInstanceState(name, req, "")
	if err != nil {
		return fmt.Errorf("falha ao solicitar mudança de estado: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- op.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("erro durante execução da operação: %w", err)
		}
		log.Printf("[LXD Provider] Ação '%s' em '%s' concluída com sucesso (Lock liberado)", action, name)
		return nil

	case <-time.After(30 * time.Second):
		return fmt.Errorf("TIMEOUT: operação demorou mais que 30s e foi abortada pelo backend. O estado do container é incerto")
	}
}

func (s *InstanceService) UpdateInstanceLimits(name string, memoryLimit string, cpuLimit string) error {
	if _, busy := s.locks.LoadOrStore(name, true); busy {
		return fmt.Errorf("LOCKED: container '%s' já está processando um comando. Tente novamente em alguns segundos", name)
	}
	defer s.locks.Delete(name)

	inst, etag, err := s.server.GetInstance(name)
	if err != nil {
		return fmt.Errorf("falha ao obter configuração atual de %s: %w", name, err)
	}

	if memoryLimit != "" {
		inst.Config["limits.memory"] = memoryLimit
	}
	if cpuLimit != "" {
		inst.Config["limits.cpu"] = cpuLimit
	}

	req := api.InstancePut{
		Config:       inst.Config,
		Devices:      inst.Devices,
		Description:  inst.Description,
		Profiles:     inst.Profiles,
		Ephemeral:    inst.Ephemeral,
		Architecture: inst.Architecture,
	}

	log.Printf("[LXD Provider] Atualizando limites para %s (RAM: %s, CPU: %s)", name, memoryLimit, cpuLimit)

	op, err := s.server.UpdateInstance(name, req, etag)
	if err != nil {
		return fmt.Errorf("falha ao solicitar atualização de limites: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- op.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("erro ao aplicar novos limites: %w", err)
		}
		log.Printf("[LXD Provider] Limites atualizados com sucesso para %s", name)
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("TIMEOUT: atualização de limites demorou muito")
	}
}

// CreateInstance cria um novo container a partir de uma imagem LOCAL.
// Agora suporta Cloud-Init (userData).
func (s *InstanceService) CreateInstance(name string, imageAlias string, limits map[string]string, userData string) error {
	if _, busy := s.locks.LoadOrStore(name, true); busy {
		return fmt.Errorf("LOCKED: já existe uma operação em andamento para criar '%s'", name)
	}
	defer s.locks.Delete(name)

	cleanImageAlias := strings.TrimPrefix(imageAlias, "images:")

	log.Printf("[LXD Provider] Verificando existência local da imagem '%s'...", cleanImageAlias)

	_, _, err := s.server.GetImageAlias(cleanImageAlias)
	if err != nil {
		return fmt.Errorf("CRITICAL: imagem '%s' não encontrada localmente no servidor LXD. O download remoto está desativado. Erro: %v", cleanImageAlias, err)
	}

	log.Printf("[LXD Provider] Imagem local confirmada. Prosseguindo com criação de '%s'", name)

	config := make(map[string]string)
	for k, v := range limits {
		config[k] = v
	}

	// Adiciona Cloud-Init (user-data) se fornecido
	if userData != "" {
		log.Printf("[LXD Provider] Injetando user-data no container '%s'", name)
		config["user.user-data"] = userData
	}

	req := api.InstancesPost{
		Name: name,
		Type: api.InstanceTypeContainer,
		Source: api.InstanceSource{
			Type:  "image",
			Alias: cleanImageAlias,
		},
		InstancePut: api.InstancePut{
			Config:   config,
			Profiles: []string{"default"},
		},
	}

	op, err := s.server.CreateInstance(req)
	if err != nil {
		return fmt.Errorf("falha ao solicitar criação de container local: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- op.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("erro durante a criação do container: %w", err)
		}
		log.Printf("[LXD Provider] Container '%s' criado com sucesso a partir de imagem local", name)
		return nil
	case <-time.After(2 * time.Minute):
		return fmt.Errorf("TIMEOUT: criação local demorou muito")
	}
}

func (s *InstanceService) DeleteInstance(name string) error {
	if _, busy := s.locks.LoadOrStore(name, true); busy {
		return fmt.Errorf("LOCKED: container '%s' já está sendo operado", name)
	}
	defer s.locks.Delete(name)

	log.Printf("[LXD Provider] Iniciando exclusão de '%s'", name)

	inst, _, err := s.server.GetInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("falha ao verificar container: %w", err)
	}

	if inst.Status == "Running" {
		log.Printf("[LXD Provider] Parando '%s' antes da exclusão...", name)
		req := api.InstanceStatePut{
			Action:  "stop",
			Timeout: -1,
			Force:   true,
		}
		op, err := s.server.UpdateInstanceState(name, req, "")
		if err != nil {
			return fmt.Errorf("falha ao parar container: %w", err)
		}
		if err := op.Wait(); err != nil {
			return fmt.Errorf("erro ao aguardar parada do container: %w", err)
		}
	}

	op, err := s.server.DeleteInstance(name)
	if err != nil {
		return fmt.Errorf("falha ao solicitar exclusão: %w", err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("erro durante a exclusão do container: %w", err)
	}

	log.Printf("[LXD Provider] Container '%s' excluído com sucesso", name)
	return nil
}

// --- Snapshot Management ---

func (s *InstanceService) ListSnapshots(instanceName string) ([]api.InstanceSnapshot, error) {
	snaps, err := s.server.GetInstanceSnapshots(instanceName)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar snapshots de %s: %w", instanceName, err)
	}
	return snaps, nil
}

func (s *InstanceService) CreateSnapshot(instanceName string, snapshotName string) error {
	if _, busy := s.locks.LoadOrStore(instanceName, true); busy {
		return fmt.Errorf("LOCKED: container '%s' já está sendo operado", instanceName)
	}
	defer s.locks.Delete(instanceName)

	log.Printf("[LXD Provider] Criando snapshot '%s' para '%s'", snapshotName, instanceName)

	req := api.InstanceSnapshotsPost{
		Name: snapshotName,
	}

	op, err := s.server.CreateInstanceSnapshot(instanceName, req)
	if err != nil {
		return fmt.Errorf("falha ao solicitar criação de snapshot: %w", err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("erro durante a criação do snapshot: %w", err)
	}

	log.Printf("[LXD Provider] Snapshot '%s' criado com sucesso", snapshotName)
	return nil
}

func (s *InstanceService) RestoreSnapshot(instanceName string, snapshotName string) error {
	if _, busy := s.locks.LoadOrStore(instanceName, true); busy {
		return fmt.Errorf("LOCKED: container '%s' já está sendo operado", instanceName)
	}
	defer s.locks.Delete(instanceName)

	log.Printf("[LXD Provider] Restaurando '%s' para snapshot '%s'", instanceName, snapshotName)

	inst, etag, err := s.server.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("falha ao obter info do container: %w", err)
	}

	if inst.Status == "Running" {
		log.Printf("[LXD Provider] Parando container para restauração...")
		stopReq := api.InstanceStatePut{
			Action:  "stop",
			Timeout: -1,
			Force:   true,
		}
		op, err := s.server.UpdateInstanceState(instanceName, stopReq, "")
		if err != nil {
			return fmt.Errorf("falha ao parar container: %w", err)
		}
		if err := op.Wait(); err != nil {
			return fmt.Errorf("erro ao aguardar parada: %w", err)
		}
		
		inst, etag, _ = s.server.GetInstance(instanceName)
	}

	req := api.InstancePut{
		Restore:      snapshotName,
		Config:       inst.Config,
		Devices:      inst.Devices,
		Profiles:     inst.Profiles,
		Description:  inst.Description,
		Architecture: inst.Architecture,
		Ephemeral:    inst.Ephemeral,
	}

	op, err := s.server.UpdateInstance(instanceName, req, etag)
	if err != nil {
		return fmt.Errorf("falha ao solicitar restauração: %w", err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("erro durante a restauração do snapshot: %w", err)
	}

	log.Printf("[LXD Provider] Restauração concluída com sucesso")
	return nil
}

func (s *InstanceService) DeleteSnapshot(instanceName string, snapshotName string) error {
	if _, busy := s.locks.LoadOrStore(instanceName, true); busy {
		return fmt.Errorf("LOCKED: container '%s' já está sendo operado", instanceName)
	}
	defer s.locks.Delete(instanceName)

	log.Printf("[LXD Provider] Deletando snapshot '%s' de '%s'", snapshotName, instanceName)

	op, err := s.server.DeleteInstanceSnapshot(instanceName, snapshotName, "")
	if err != nil {
		return fmt.Errorf("falha ao solicitar exclusão de snapshot: %w", err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("erro durante a exclusão do snapshot: %w", err)
	}

	return nil
}

// --- Port Forwarding (Proxy Devices) ---

func (s *InstanceService) AddProxyDevice(instanceName string, hostPort int, containerPort int, protocol string) error {
	if _, busy := s.locks.LoadOrStore(instanceName, true); busy {
		return fmt.Errorf("LOCKED: container '%s' já está sendo operado", instanceName)
	}
	defer s.locks.Delete(instanceName)

	if err := s.CheckPortAvailability(hostPort); err != nil {
		return err
	}

	if protocol != "tcp" && protocol != "udp" {
		return fmt.Errorf("protocolo inválido: %s. Use 'tcp' ou 'udp'", protocol)
	}

	deviceName := fmt.Sprintf("proxy-%d", hostPort)
	log.Printf("[LXD Provider] Adicionando Port Forward: Host:%d -> Container:%d (%s)", hostPort, containerPort, protocol)

	inst, etag, err := s.server.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("falha ao obter instancia: %w", err)
	}

	if inst.Devices == nil {
		inst.Devices = make(map[string]map[string]string)
	}

	inst.Devices[deviceName] = map[string]string{
		"type":    "proxy",
		"listen":  fmt.Sprintf("%s:0.0.0.0:%d", protocol, hostPort),
		"connect": fmt.Sprintf("%s:127.0.0.1:%d", protocol, containerPort),
		"bind":    "host",
	}

	req := api.InstancePut{
		Config:       inst.Config,
		Devices:      inst.Devices, 
		Profiles:     inst.Profiles,
		Description:  inst.Description,
		Architecture: inst.Architecture,
		Ephemeral:    inst.Ephemeral,
	}

	op, err := s.server.UpdateInstance(instanceName, req, etag)
	if err != nil {
		return fmt.Errorf("falha ao adicionar porta: %w", err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("erro ao aplicar proxy device: %w", err)
	}

	log.Printf("[LXD Provider] Porta %d adicionada com sucesso", hostPort)
	return nil
}

func (s *InstanceService) RemoveProxyDevice(instanceName string, hostPort int) error {
	if _, busy := s.locks.LoadOrStore(instanceName, true); busy {
		return fmt.Errorf("LOCKED: container '%s' já está sendo operado", instanceName)
	}
	defer s.locks.Delete(instanceName)

	deviceName := fmt.Sprintf("proxy-%d", hostPort)
	log.Printf("[LXD Provider] Removendo Port Forward: %s", deviceName)

	inst, etag, err := s.server.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("falha ao obter instancia: %w", err)
	}

	if _, exists := inst.Devices[deviceName]; !exists {
		return fmt.Errorf("porta %d não encontrada (device %s)", hostPort, deviceName)
	}

	delete(inst.Devices, deviceName)

	req := api.InstancePut{
		Config:       inst.Config,
		Devices:      inst.Devices,
		Profiles:     inst.Profiles,
		Description:  inst.Description,
		Architecture: inst.Architecture,
		Ephemeral:    inst.Ephemeral,
	}

	op, err := s.server.UpdateInstance(instanceName, req, etag)
	if err != nil {
		return fmt.Errorf("falha ao remover porta: %w", err)
	}

	if err := op.Wait(); err != nil {
		return fmt.Errorf("erro ao remover proxy device: %w", err)
	}

	log.Printf("[LXD Provider] Porta %d removida com sucesso", hostPort)
	return nil
}

// --- File System (Explorer) ---

// ListFiles lista arquivos e diretórios em um caminho específico.
func (s *InstanceService) ListFiles(instanceName string, path string) ([]FileEntry, error) {
	// First, check if the path itself is a directory
	content, resp, err := s.server.GetInstanceFile(instanceName, path)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler caminho '%s': %w", path, err)
	}

	if content != nil {
		defer content.Close()
	}

	if resp.Type != "directory" {
		return nil, fmt.Errorf("o caminho '%s' não é um diretório (tipo retornado: %s)", path, resp.Type)
	}

	var entries []FileEntry
	for _, entryName := range resp.Entries {
		entryPath := path
		if entryPath == "/" {
			entryPath += entryName
		} else {
			entryPath += "/" + entryName
		}

		_, entryResp, err := s.server.GetInstanceFile(instanceName, entryPath) // get info for each entry
		if err != nil {
			log.Printf("Warning: Failed to get info for '%s': %v", entryPath, err)
			// If we can't get info for an entry, we can default to file or skip. Skipping for now.
			continue
		}
		entries = append(entries, FileEntry{
			Name: entryName,
			Type: entryResp.Type,
		})
	}
	return entries, nil
}

// DownloadFile baixa o conteúdo de um arquivo.
func (s *InstanceService) DownloadFile(instanceName string, path string) (io.ReadCloser, int64, error) {
	content, resp, err := s.server.GetInstanceFile(instanceName, path)
	if err != nil {
		return nil, 0, fmt.Errorf("falha ao baixar arquivo: %w", err)
	}

	if resp.Type == "directory" {
		content.Close()
		return nil, 0, fmt.Errorf("caminho '%s' é um diretório", path)
	}

	// Size não disponível na struct desta versão, retornamos -1
	return content, -1, nil
}

// UploadFile envia um arquivo para o container.
func (s *InstanceService) UploadFile(instanceName string, path string, content io.ReadSeeker) error {
	// Bufferizar conteúdo para garantir Seek e Size corretos para o LXD
	data, err := io.ReadAll(content)
	if err != nil {
		return fmt.Errorf("falha ao ler conteúdo do upload: %w", err)
	}
	
	readSeeker := strings.NewReader(string(data))

	args := lxd.InstanceFileArgs{
		UID:     0,
		GID:     0,
		Mode:    0644,
		Type:    "file",
		Content: readSeeker,
		WriteMode: "overwrite", // Força sobrescrita explícita
	}

	log.Printf("[LXD Provider] Uploading file to '%s:%s' (%d bytes)", instanceName, path, len(data))
	
	err = s.server.CreateInstanceFile(instanceName, path, args)
	if err != nil {
		return fmt.Errorf("falha no upload: %w", err)
	}

	return nil
}

// DeleteFile deleta um arquivo ou diretório.
func (s *InstanceService) DeleteFile(instanceName string, path string) error {
	log.Printf("[LXD Provider] Deletando arquivo '%s:%s'", instanceName, path)
	
	err := s.server.DeleteInstanceFile(instanceName, path)
	if err != nil {
		return fmt.Errorf("falha ao deletar arquivo: %w", err)
	}
	return nil
}