# ⚡ Axion Control Plane
### HPC-First Container & Virtualization Platform

> **"Axion não gerencia máquinas. Ele domina o hardware."**

O **Axion** é um control plane de containers e virtualização focado em **performance extrema, baixa latência e HPC (High Performance Computing)**. Ele nasce para ser rápido, visual, otimizado e agressivo, eliminando a gordura dos painéis tradicionais.

**Axion não é um fork de Proxmox.** É uma arquitetura moderna, assíncrona, orientada a eventos e desenhada para escalar.

---

## 🚀 Status do Projeto

![Status](https://img.shields.io/badge/Status-Active-success)
![Version](https://img.shields.io/badge/Version-1.0_RC-blue)
![Build](https://img.shields.io/badge/Build-Passing-brightgreen)

✅ **Backend & Frontend Operacionais**
✅ **Containers LXC & VMs KVM em Produção**
✅ **Cluster Mode (TLS) Ativo**

O Axion já é um Control Plane completo, oferecendo ciclo de vida total de instâncias, orquestração de rede e armazenamento, e ferramentas de operação "Day 2" (Terminal, Arquivos, Logs).

---

## 🧠 Filosofia

* **Performance First:** Cada milissegundo conta. Arquitetura feita para HPC.
* **Zero Bloatware:** Sem agentes pesados. O Axion roda leve e deixa o hardware para o workload.
* **Assíncrono & Real-Time:** Nada de "refresh na página". Tudo é atualizado via WebSockets multiplexados.
* **No Vendor Lock-in:** Baseado em padrões abertos (LXC/LXD/KVM).
* **Visual Enterprise:** Interface "Dark Mode" densa e informativa.

---

## 🏗️ Stack Tecnológico

### Backend (The Engine)
* **Core:** Go (Golang) 1.25+
* **API:** Gin Framework (High Performance HTTP)
* **Database:** SQLite (WAL Mode) com auto-recovery para persistência de Jobs e Schedules.
* **Orquestração:** LXD via Socket Unix (Local) ou TLS (Cluster).
* **Async System:** Worker Pool com filas persistentes, retry exponencial e locks por instância para evitar race conditions.

### Frontend (The Cockpit)
* **Framework:** Next.js 16 (App Router)
* **UI Library:** Tailwind CSS + Lucide Icons + Sonner.
* **Features:** Sidebar Navigation, Monaco Editor integrado, Terminal xterm.js, Telemetria em tempo real (Recharts).

---

## 🛠️ Instalação e Execução

### Pré-requisitos
* **Linux** (Ubuntu 22.04/24.04 recomendado)
* **Go 1.25+**
* **Node.js 20+** e NPM
* **LXD** instalado e inicializado (`lxd init`)

### 1. Setup Inicial e Backend
```bash
# 1. Prepare as imagens do LXC/LXD (Opcional, popula o cache local)
chmod +x preload_full.sh
./preload_full.sh

# 2. Instale as dependências do Go
go mod tidy

# 3. Inicie o Control Plane (Backend)
go run main.go
```
*O Backend iniciará na porta `8500`.*

### 2. Setup do Frontend
Em um novo terminal:
```bash
cd frontend

# 1. Instale as dependências
npm install

# 2. Inicie o servidor de desenvolvimento
npm run dev
```
*O Dashboard estará acessível em `http://localhost:3000`.*

---

## ⚡ Funcionalidades (Implementadas)

### 🖥️ Compute & Orquestração
* **LXC & KVM:** Suporte nativo a Containers (`container`) e Virtual Machines (`virtual-machine`).
* **Cloud-Init:** Injeção automática de `user-data` para configuração inicial de rede e pacotes.
* **Resource Limits:** Controle rígido de vCPU e RAM por instância.
* **Global Quotas:** Sistema de governança que impede over-provisioning do host (Limites globais hardcoded para segurança).
* **Cluster Awareness:** Conexão segura via TLS para gerenciamento de múltiplos nós LXD.
* **Host Telemetry:** Monitoramento em tempo real de CPU, RAM, Disco e Rede do servidor físico via WebSocket.

### 💾 Storage & Arquivos
* **Snapshots (Time Machine):** Criar, Restaurar e Deletar backups instantâneos das instâncias.
* **Axion Explorer:** Gerenciador de arquivos completo (Listar, Upload, Download, Deletar).
* **Integrated IDE:** Edição de arquivos de configuração com **Monaco Editor** direto no navegador.
* **Streaming Upload/Download:** Transferência eficiente de arquivos grandes.

### 🌐 Rede & Conectividade
* **Port Forwarding:** Criação de Proxy Devices para mapear portas do Host (10000-60000) para Containers/VMs (TCP/UDP).
* **Network Manager:** Gestão completa de Bridges e Subnets.
* **Boot Logs:** Acesso aos logs de console da instância para debug.

### 🛡️ Segurança & Governança
* **Autenticação:** JWT com expiração de 24h e suporte a rotação de segredos via ENV.
* **Web Terminal:** Acesso root interativo via WebSocket binário (xterm.js) com suporte a redimensionamento de janela.
* **Job System:** Fila de tarefas persistente em SQLite com recuperação automática de falhas e sistema de retry inteligente.

---

## ⚙️ Automação (Scheduler)

O Axion possui um **Scheduler Integrado** persistente:
1.  **Cron Expressions:** Agendamento de tarefas recorrentes usando sintaxe padrão Cron.
2.  **Persistence:** Agendamentos salvos no banco SQLite, sobrevivendo a reinícios.
3.  **Job Dispatch:** O scheduler dispara Jobs para a fila do Worker Pool automaticamente.

---

## 🧪 Ambientes de Uso

* **HPC Labs:** Clusters de alta densidade para cálculos científicos.
* **Game Servers:** Hospedagem de baixa latência (Minecraft, CS2, Rust).
* **DevOps:** Ambientes de CI/CD efêmeros e reprodutíveis.
* **Homelabs:** A alternativa leve e moderna ao Proxmox.

---

## 📜 Licenciamento

O **Axion NÃO é open-source completo**.

* **Personal:** Gratuito para uso pessoal e aprendizado.
* **Enterprise:** Licença comercial para uso em produção/revenda.

---

## 🧭 Roadmap

* [x] **v1.0 (Atual):** Containers/VMs, Rede, Storage, Terminal, Cloud-Init, Cluster Mode, Scheduler.
* [ ] **v1.1:** Firewall por instância e Security Groups.
* [ ] **v1.2:** Multi-tenant (SaaS Mode) e Billing Hooks.
* [ ] **v2.0:** HA (Alta Disponibilidade) e Live Migration.

---

#### Desenvolvido por Pedrortxdev
> *High Performance Computing for the Modern Era.*
