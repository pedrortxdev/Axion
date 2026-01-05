# AxHV Daemon - API Documentation

*Documentação completa do AxHV Daemon para gerenciamento de microVMs Firecracker.*

---

## Índice

1. [Visão Geral](#visão-geral)
2. [Conexão](#conexão)
   - [Unix Socket (Local)](#unix-socket-local)
   - [TCP (Cluster)](#tcp-cluster)
3. [Arquitetura de Rede](#arquitetura-de-rede)
4. [Métodos RPC](#métodos-rpc)
5. [Ciclo de Vida da VM](#ciclo-de-vida-da-vm)
6. [Free Tier Network](#free-tier-network)
7. [Validação de Entrada](#validação-de-entrada)
8. [Persistência](#persistência)
9. [Referência de Mensagens](#referência-de-mensagens)

---

## Visão Geral

O **AxHV Daemon** é um servidor gRPC de alta performance para gerenciamento de microVMs Firecracker.

| Característica | Valor |
|---------------|-------|
| Protocolo | gRPC / Protobuf v3 |
| Package | `axhv` |
| Service | `VmService` |
| Default Socket | `/tmp/axhv.sock` |

### Iniciar o Daemon

```bash
# Modo local (Unix Socket)
sudo ./axhv-daemon

# Modo cluster (TCP)
export AXHV_CLUSTER_TOKEN="seu-token-secreto"
sudo -E ./axhv-daemon --listen 0.0.0.0:50051

# Ver ajuda
./axhv-daemon --help
```

### Flags CLI

| Flag | Env Variable | Descrição | Default |
|------|--------------|-----------|---------|
| `--listen` / `-l` | - | Endereço para escutar | `/tmp/axhv.sock` |
| `--token` / `-t` | `AXHV_CLUSTER_TOKEN` | Token de autenticação (TCP) | - |
| `--r2-account-id` | `R2_ACCOUNT_ID` | ID da conta Cloudflare R2 | - |
| `--r2-bucket` | `R2_BUCKET` | Nome do bucket R2 | - |

---

## Conexão

### Unix Socket (Local)

O modo padrão. Socket criado com permissões `0600` (somente root).

```bash
# grpcurl
sudo grpcurl -plaintext unix:///tmp/axhv.sock axhv.VmService/ListVms

# Python
import grpc
channel = grpc.insecure_channel('unix:///tmp/axhv.sock')

# Go
conn, _ := grpc.Dial("unix:///tmp/axhv.sock", grpc.WithInsecure())
```

### TCP (Cluster)

Para comunicação entre nós de um cluster. Requer autenticação via header `Authorization: Bearer <token>`.

```bash
# Iniciar daemon em modo TCP
export AXHV_CLUSTER_TOKEN="meu-token-super-secreto"
sudo -E ./axhv-daemon --listen 0.0.0.0:50051

# Conectar com token
grpcurl -plaintext \
  -H "Authorization: Bearer meu-token-super-secreto" \
  192.168.1.100:50051 \
  axhv.VmService/ListVms
```

> ⚠️ **Sem token configurado, qualquer um pode controlar o host!**

---

## Arquitetura de Rede

```
┌─────────────────────────────────────────────────────────────────┐
│ Host                                                             │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │  axhv-br0 (172.16.0.1/24) - Bridge Virtual                 │ │
│  │    ├── axhv-vm-1 (TAP) ──→ VM-1 (172.16.0.2)              │ │
│  │    ├── axhv-vm-2 (TAP) ──→ VM-2 (172.16.0.3)              │ │
│  │    └── axhv-vm-N (TAP) ──→ VM-N (172.16.0.N+1)            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                            │                                     │
│                     NAT (iptables MASQUERADE)                    │
│                            │                                     │
│                      [ Internet ]                                │
└─────────────────────────────────────────────────────────────────┘
```

### Componentes Automáticos

| Componente | Descrição |
|------------|-----------|
| **TAP** | Criada automaticamente como `axhv-{vm_id}` |
| **Bridge** | `axhv-br0` criada na primeira VM |
| **NAT** | Masquerade configurado para saída internet |
| **Disco** | Clonado para `/var/lib/axhv/instances/{id}.ext4` |

---

## Métodos RPC

| Método | Descrição | Requer VM Rodando? |
|--------|-----------|-------------------|
| `CreateVm` | Cria e inicia uma nova VM | N/A |
| `StopVm` | Para uma VM graciosamente | ✅ |
| `StartVm` | Inicia uma VM parada | ❌ (parada) |
| `PauseVm` | Suspende vCPUs (congela) | ✅ |
| `ResumeVm` | Retoma vCPUs suspensas | ✅ (pausada) |
| `RebootVm` | Reinicia VM (hard: stop+start) | ✅ |
| `ResizeDisk` | Aumenta tamanho do disco | ❌ (parada) |
| `DeleteVm` | Remove VM permanentemente | Qualquer |
| `GetHostStats` | Estatísticas do host | N/A |
| `ListVms` | Lista VMs ativas | N/A |

---

## Ciclo de Vida da VM

```
                    ┌──────────────┐
                    │   CreateVm   │
                    └──────┬───────┘
                           │
                           ▼
   ┌───────────┐    ┌──────────────┐    ┌───────────┐
   │  PauseVm  │───▶│   RUNNING    │◀───│ ResumeVm  │
   └───────────┘    └──────┬───────┘    └───────────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
              ▼            ▼            ▼
       ┌──────────┐  ┌──────────┐  ┌──────────┐
       │  StopVm  │  │ RebootVm │  │ PauseVm  │
       └────┬─────┘  └──────────┘  └────┬─────┘
            │                           │
            ▼                           ▼
     ┌──────────────┐            ┌──────────────┐
     │   STOPPED    │            │    PAUSED    │
     │ (config ok)  │            │   (frozen)   │
     └──────┬───────┘            └──────────────┘
            │
    ┌───────┴───────┐
    │               │
    ▼               ▼
┌─────────┐   ┌────────────┐
│ StartVm │   │ ResizeDisk │
└─────────┘   └────────────┘
```

### StopVm

Para a VM graciosamente (SIGTERM, depois SIGKILL).
**O config é preservado** em `vms.json` com `pid: 0` para que `StartVm` possa reiniciar.

```bash
grpcurl -plaintext -d '{"id": "vm-1"}' unix:///tmp/axhv.sock axhv.VmService/StopVm
```

### StartVm

Inicia uma VM que foi parada anteriormente. Usa o config salvo em `vms.json`.

```bash
grpcurl -plaintext -d '{"id": "vm-1"}' unix:///tmp/axhv.sock axhv.VmService/StartVm
```

### RebootVm

Executa um **hard reboot**: para o processo Firecracker e inicia novamente.
Não usa Ctrl+Alt+Del (não funciona bem com Firecracker).

```bash
grpcurl -plaintext -d '{"id": "vm-1"}' unix:///tmp/axhv.sock axhv.VmService/RebootVm
```

### PauseVm / ResumeVm

Congela/descongela vCPUs via API do Firecracker (`PATCH /vm {"state": "Pause/Resume"}`).
A VM continua consumindo memória, mas para de executar.

```bash
# Pausar
grpcurl -plaintext -d '{"id": "vm-1"}' unix:///tmp/axhv.sock axhv.VmService/PauseVm

# Retomar
grpcurl -plaintext -d '{"id": "vm-1"}' unix:///tmp/axhv.sock axhv.VmService/ResumeVm
```

### ResizeDisk

Aumenta o tamanho do disco de uma VM. **A VM deve estar parada.**

1. Expande o arquivo (truncate - instantâneo, sparse file)
2. Executa `e2fsck -pf` (verifica filesystem)
3. Executa `resize2fs` (expande ext4)

```bash
# Parar VM
grpcurl -plaintext -d '{"id": "vm-1"}' unix:///tmp/axhv.sock axhv.VmService/StopVm

# Aumentar para 10GB
grpcurl -plaintext -d '{"id": "vm-1", "new_size_gb": 10}' unix:///tmp/axhv.sock axhv.VmService/ResizeDisk

# Iniciar
grpcurl -plaintext -d '{"id": "vm-1"}' unix:///tmp/axhv.sock axhv.VmService/StartVm
```

> **Nota:** Só permite aumentar, não diminuir.

### GetHostStats

Retorna estatísticas de disco do host (partição de `/var/lib/axhv`).

```bash
grpcurl -plaintext unix:///tmp/axhv.sock axhv.VmService/GetHostStats
```

Resposta:
```json
{
  "diskTotalMib": "953869",
  "diskUsedMib": "272034",
  "diskFreeMib": "681835",
  "vmCount": 2
}
```

---

## Free Tier Network

Controle de rede para VMs Free Tier com limites rigorosos.

### Limites Hardcoded (Segurança)

| Recurso | Limite |
|---------|--------|
| Portas TCP | 3 por VM |
| Portas UDP | 1 por VM |
| Bandwidth | Configurável via `bandwidth_limit_mbps` |

> Se exceder os limites, a criação da VM **falha** (erro 400).

### Rate Limiting (Traffic Control)

Usa `tc` (Linux Traffic Control) com:

- **Download (Internet → VM):** TBF (Token Bucket Filter) na raiz da TAP
- **Upload (VM → Internet):** Policer ingress com `drop` nos pacotes excedentes

```bash
# Internamente executado:
tc qdisc add dev axhv-vm-1 root tbf rate 10mbit burst 100kbit latency 400ms
tc qdisc add dev axhv-vm-1 handle ffff: ingress
tc filter add dev axhv-vm-1 parent ffff: protocol ip u32 match u32 0 0 \
   police rate 10mbit burst 100kbit drop flowid :1
```

### Port Forwarding (iptables DNAT)

Redireciona portas do host para portas da VM.

```bash
# Internamente executado:
iptables -t nat -A PREROUTING -p tcp --dport 8080 -j DNAT --to-destination 172.16.0.2:80
iptables -t nat -A OUTPUT -p tcp --dport 8080 -j DNAT --to-destination 172.16.0.2:80
iptables -A FORWARD -p tcp -d 172.16.0.2 --dport 80 -j ACCEPT
```

### Exemplo Completo

```bash
grpcurl -plaintext -d '{
  "id": "free-vm",
  "vcpu": 1,
  "memory_mib": 128,
  "kernel_path": "/home/daniel/Documentos/AxHV2/vmlinux-distro",
  "rootfs_path": "/home/daniel/Documentos/AxHV2/ubuntu-rootfs.ext4",
  "guest_ip": "172.16.0.10",
  "guest_gateway": "172.16.0.1",
  "bandwidth_limit_mbps": 10,
  "port_map_tcp": {"2222": 22, "8080": 80, "8443": 443}
}' unix:///tmp/axhv.sock axhv.VmService/CreateVm

# Acessar SSH via host
ssh -p 2222 root@localhost
```

### GetVmStats

Retorna estatísticas em tempo real (CPU, Memória, Rede, Disco).
Valores de rede são acumulados desde o boot (bytes absolutos do host).

- **net_rx_bytes**: Bytes recebidos pela interface host (Upload da VM)
- **net_tx_bytes**: Bytes enviados pela interface host (Download da VM)

```bash
grpcurl -plaintext -d '{"id": "vm-1"}' unix:///tmp/axhv.sock axhv.VmService/GetVmStats
```

Resposta:
```json
{
  "cpuUsageUs": "0",
  "memoryUsedBytes": "0",
  "netRxBytes": "5242880",
  "netTxBytes": "10485760",
  "diskAllocatedBytes": "1073741824"
}
```

---

## Validação de Entrada

Todas as entradas são validadas rigorosamente antes de processar.

### VM ID

| Regra | Valor |
|-------|-------|
| Tamanho | 1-32 caracteres |
| Caracteres | `a-z`, `A-Z`, `0-9`, `-`, `_` |
| Primeiro char | Alfanumérico obrigatório |
| Proibido | `..`, `/`, `\`, `;`, `` ` ``, `$()` |

```bash
# Válidos
"my-vm-1", "test_vm", "VM123"

# Inválidos (rejeitados)
"../etc/passwd"   # Path traversal
"-invalid"        # Começa com -
"vm;rm -rf /"     # Injection
```

### Guest IP

| Regra | Valor |
|-------|-------|
| Subnet | Deve ser `172.16.0.x` |
| Proibido | `.0` (network), `.1` (gateway), `.255` (broadcast) |

```bash
# Válidos
"172.16.0.2", "172.16.0.100", "172.16.0.254"

# Inválidos
"192.168.1.1"    # Subnet errada
"172.16.0.1"     # Gateway reservado
"172.16.0.255"   # Broadcast
```

### Recursos

| Campo | Mínimo | Máximo |
|-------|--------|--------|
| `vcpu` | 1 | 32 |
| `memory_mib` | 128 | 32768 (32 GB) |
| `disk_size_gb` | - | 1024 (1 TB) |

---

## Persistência

### Arquivo de Estado

**Localização:** `/var/lib/axhv/vms.json`

Estrutura:
```json
{
  "vm-1": {
    "id": "vm-1",
    "vcpu": 1,
    "memory_mib": 128,
    "kernel_path": "/path/to/vmlinux",
    "rootfs_path": "/var/lib/axhv/instances/vm-1.ext4",
    "disk_size_gb": 5,
    "guest_ip": "172.16.0.2",
    "guest_gateway": "172.16.0.1",
    "boot_args": "",
    "tap_name": "axhv-vm-1",
    "socket_path": "/tmp/axhv-vm-1.sock",
    "pid": 12345
  }
}
```

- `pid: 0` significa VM parada
- `pid > 0` significa VM (teoricamente) rodando

### Recuperação ao Iniciar

Quando o daemon inicia:

1. Lê `vms.json`
2. Para cada VM com `pid > 0`:
   - Tenta reconectar ao processo existente
   - Se o processo morreu mas o disco existe → auto-reinicia
   - Se o disco não existe → remove do JSON

### Discos

**Localização:** `/var/lib/axhv/instances/{id}.ext4`

- Clonados automaticamente do `rootfs_path` original
- Formato ext4 RAW
- Podem ser redimensionados via `ResizeDisk`

### Logs

**Localização:** `/var/log/axhv/vm-{id}.out`

Stdout/stderr do processo Firecracker são redirecionados para estes arquivos.

---

## Referência de Mensagens

### CreateVmRequest

| Campo | Tipo | Obrigatório | Descrição |
|-------|------|-------------|-----------|
| `id` | string | ✅ | ID único (1-32 chars, alfanum + `-_`) |
| `vcpu` | uint32 | ✅ | vCPUs (1-32) |
| `memory_mib` | uint32 | ✅ | RAM em MiB (128-32768) |
| `kernel_path` | string | ✅ | Caminho absoluto do kernel |
| `rootfs_path` | string | ✅* | Caminho da imagem base |
| `template` | string | ✅* | Template R2 (alternativa) |
| `disk_size_gb` | uint32 | ❌ | Tamanho do disco em GB |
| `guest_ip` | string | ✅ | IP da VM (172.16.0.x) |
| `guest_gateway` | string | ✅ | Gateway (172.16.0.1) |
| `boot_args` | string | ❌ | Args extras para kernel |
| `bandwidth_limit_mbps` | uint32 | ❌ | Limite de banda (0=ilimitado) |
| `port_map_tcp` | map | ❌ | Port forward TCP (max 3) |
| `port_map_udp` | map | ❌ | Port forward UDP (max 1) |

> *Use `rootfs_path` OU `template`, não ambos.

### VmResponse

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `success` | bool | Operação bem-sucedida |
| `message` | string | Mensagem descritiva |
| `vm_id` | string | ID da VM afetada |

### HostStatsResponse

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `disk_total_mib` | uint64 | Espaço total em MiB |
| `disk_used_mib` | uint64 | Espaço usado em MiB |
| `disk_free_mib` | uint64 | Espaço livre em MiB |
| `vm_count` | uint32 | VMs ativas |

### VmInfo

| Campo | Tipo | Descrição |
|-------|------|-----------|
| `id` | string | ID da VM |
| `pid` | uint32 | PID do processo Firecracker |
| `socket_path` | string | Caminho do socket API |

---

## Performance

| Métrica | Valor Típico |
|---------|--------------|
| Spawn + Config | ~49ms |
| Boot Ubuntu | ~2.5s |
| Latência de rede | <1ms |
| Clone de disco | ~2-5s (depende do tamanho) |

---

*Documentação gerada automaticamente do código-fonte AxHV v0.1.0*