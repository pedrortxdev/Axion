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
✅ **Containers LXC em Produção**
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
* **Core:** Go (Golang) 1.22+
* **API:** Gin Framework (High Performance HTTP)
* **Database:** SQLite (WAL Mode) com auto-recovery.
* **Orquestração:** LXD via Socket Unix (Local) ou TLS (Cluster).
* **Async System:** Worker Pool com filas persistentes, retry exponencial e locks por instância.

### Frontend (The Cockpit)
* **Framework:** Next.js 14 (App Router)
* **UI Library:** Tailwind CSS + Radix UI + Lucide Icons.
* **Features:** Sidebar Navigation, Monaco Editor integrado, Terminal xterm.js, Telemetria em tempo real (Recharts).

---

## ⚡ Funcionalidades (O que já funciona)

### 🖥️ Compute & Orquestração
* **Containers LXC:** Criação, Start, Stop, Restart e Delete instantâneos.
* **Cloud-Init Templates:** Deploy automático de stacks (Docker Host, Web Server) via *user-data*.
* **Hotplug de Recursos:** Ajuste dinâmico de vCPU e RAM sem reiniciar.
* **Cluster Awareness:** Suporte a múltiplos nós via conexão TLS segura.
* **Host Telemetry:** Monitoramento visual de CPU/RAM/Disk/Network do servidor físico ("Telemetry Deck").

### 💾 Storage & Arquivos
* **Snapshots (Time Machine):** Criar, Restaurar e Deletar backups instantâneos (ZFS/LVM).
* **Axion Explorer:** Gerenciador de arquivos completo no navegador.
* **Integrated IDE:** Edição de arquivos de configuração com **Monaco Editor** (VS Code engine) e syntax highlighting.
* **Transfer:** Upload e Download de arquivos direto pelo painel.

### 🌐 Rede & Conectividade
* **Network Manager:** Criação e gestão de Bridges e Redes virtuais.
* **Port Forwarding:** Mapeamento visual de portas (Host -> Container) usando Proxy Devices.
* **Boot Logs:** Visualizador "Matrix" de logs do console para debug de inicialização.

### 🛡️ Segurança & Governança
* **Autenticação:** JWT com rotação e expiração de 24h.
* **Resource Quotas:** Tetos globais de CPU e RAM para proteger o Host.
* **Web Terminal:** Acesso root via WebSocket binário (xterm.js) sem necessidade de SSH exposto.

---

## ⚙️ Automação (Job System)

O coração do Axion é um motor de Jobs resiliente:
1.  **Estados:** `PENDING` -> `IN_PROGRESS` -> `COMPLETED` / `FAILED`.
2.  **Resiliência:** Se o servidor reiniciar, jobs travados são recuperados automaticamente.
3.  **Cron Scheduler:** Agendamento de tarefas recorrentes (ex: Snapshots diários).

---

## 📸 Screenshots

*(Espaço reservado para as imagens do Dashboard, Terminal e Wizard)*

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

* [x] **v1.0 (Atual):** Containers, Rede, Storage, Terminal, Cloud-Init, Cluster Mode.
* [ ] **v1.1:** Suporte completo a KVM/VMs (Windows/Linux).
* [ ] **v1.2:** Firewall por instância e Security Groups.
* [ ] **v2.0:** Multi-tenant (SaaS Mode), Billing Hooks e HA (Alta Disponibilidade).

---

#### Desenvolvido por Pedrortxdev
> *High Performance Computing for the Modern Era.*
