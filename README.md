# ⚡ Axion Control Plane
### HPC-First Container & Virtualization Platform (Proxmox Killer)

> **Axion** é um **control plane de containers e virtualização focado em performance extrema, baixa latência e HPC**.  
Ele nasce para ser **rápido, visual, otimizado e agressivo**, sem a gordura dos painéis tradicionais.

Axion **não é um fork de Proxmox**.  
Ele é uma **arquitetura moderna, assíncrona, em tempo real e feita para escalar**.

---

## 🚀 Status Atual do Projeto

✅ **Projeto ATIVO**  
✅ Backend funcional  
✅ Frontend funcional  
✅ Containers rodando em produção  
✅ Terminal web, snapshots, port-forward, cloud-init, arquivos, tudo funcionando  

> O Axion **já é um Control Plane completo para containers LXC.**

---

## 🎯 Foco do Projeto

- HPC (High Performance Computing)
- Game Servers de altíssima densidade
- Renderização
- IA / Machine Learning
- Ambientes científicos
- Clusters privados
- Infraestrutura de alto desempenho

---

## 🧠 Filosofia

- **Performance acima de tudo**
- **Arquitetura assíncrona**
- **Zero desperdício de recurso**
- **Visual moderno**
- **Controle total do host**
- **Sem dependência de cloud externa**
- **Nada de vendor lock-in**

---

## 🏗️ Arquitetura Atual (REAL)

### 🔧 Backend (Control Plane)
- **Linguagem:** Go 1.22+
- **Framework HTTP:** Gin
- **Banco de Dados:** SQLite em WAL Mode
- **Autenticação:** JWT (24h)
- **WebSocket:** Telemetria + Terminal + Eventos
- **Execução de Jobs:** Sistema assíncrono com workers, retry e backoff exponencial

---

### ⚙️ Sistema de Jobs (Async Engine)

- Worker Pool com concorrência
- Fila persistida em SQLite
- Estados:
  - PENDING
  - IN_PROGRESS
  - COMPLETED
  - FAILED
- Retry automático com backoff exponencial
- Recovery automático de jobs travados no boot
- Locks por container (evita ações concorrentes)

---

### 📦 Virtualização Atual

✅ **Containers LXC (100% funcional)**  
⚠️ VMs (KVM) **planejado para v2.0**

Atualmente:
- Containers compartilham o kernel do host
- Extremamente mais eficientes que VMs
- Ideal para HPC, game servers e workloads massivos

---

### 📊 Funcionalidades Implementadas

✅ Criação de Containers  
✅ Start / Stop / Restart  
✅ Monitoramento em tempo real (CPU, RAM)  
✅ Terminal Web interativo  
✅ Ajuste dinâmico de CPU e RAM  
✅ Quota Global de Recursos (Governança)  
✅ Snapshots (Create, Restore, Delete)  
✅ Port Forwarding com validação de portas  
✅ Gerenciador de Arquivos  
✅ Editor de Arquivos com Monaco Editor  
✅ Autenticação JWT  
✅ API protegida  
✅ WebSockets seguros  
✅ Telemetria em tempo real  
✅ Job System resiliente  
✅ Locks por instância  
✅ Fallback de imagem local  
✅ Cloud-Init 

---

## 🔐 Segurança

- JWT com expiração
- Middleware em todas as rotas críticas
- Proteção de WebSocket por token
- Validação de portas no port-forward
- Quotas globais de CPU e RAM
- Prevenção de colisão de nomes
- Locks por container

---

## 📦 Snapshots (Backups)

- Criar snapshot
- Restaurar snapshot (com stop automático se necessário)
- Deletar snapshot
- Tudo operando via Jobs assíncronos
- Interface completa no painel

---

## 🌐 Rede

- Port Forwarding por container
- Validação automática de conflitos
- Faixa de portas segura (10000–60000)
- Proxy Device dinâmico no LXD

---

## 🖥️ Frontend

- **Framework:** Next.js 16
- **Design:** Enterprise Dark
- **Features:**
  - Login JWT
  - Dashboard de instâncias
  - Wizard de criação
  - Terminal web
  - Ajuste de recursos
  - Drawer de Snapshots
  - Drawer de Arquivos
  - Editor Monaco
  - Activity Log em tempo real
  - Feedback visual de jobs
  - Toasts e confirmação de ações

---

## 📈 Governança de Recursos

- Teto global de recursos:
  - CPU total do host
  - RAM total do host
- Nenhuma instância pode ultrapassar o limite físico
- Todas as requisições passam por pré-validação

---

## 📡 Comunicação em Tempo Real

- WebSocket Multiplexado:
  - Telemetria de CPU/RAM
  - Eventos de Jobs
  - Terminal interativo
- Event Bus interno desacoplado dos Workers

---

## 🧪 Ambientes de Uso

- Laboratórios de HPC
- Provedores de Game Server
- Clusters privados
- Infraestrutura própria

---

## 📜 Licenciamento

O **Axion NÃO é open-source completo**.

Modelo de licenciamento:
- Uso pessoal (Personal)
- Uso profissional (Enterprise)

---

## 🧭 Roadmap (Próximas Fases)

- [ ] Gerenciamento de usuários (multi-tenant)
- [ ] Firewall por instância
- [ ] Estatísticas históricas
- [ ] Backup externo
- [ ] Suporte a KVM/VMs 
- [ ] Multi-node control plane (v2.0)
- [ ] Alta disponibilidade
- [ ] Scheduler de HPC

---

## 👑 Autor

Axion foi criado para ser:
- Um **Hypervisor moderno**
- Um **Painel HPC de nova geração**

> **“Axion não gerencia máquinas. Ele domina o hardware.”**
