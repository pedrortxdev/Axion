# ⚡ Axion Control Plane

<div align="center">

  [![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go)](https://golang.org/)
  [![Next.js](https://img.shields.io/badge/Next.js-16-000000?style=for-the-badge&logo=next.js)](https://nextjs.org/)
  [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-DB4B8B?style=for-the-badge&logo=postgresql&logoColor=white)](https://www.postgresql.org/)
  [![MIT License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](LICENSE)
  [![LXD](https://img.shields.io/badge/LXD-Container%20Platform-326DE6?style=for-the-badge&logo=linuxcontainers)](https://linuxcontainers.org/lxd/)

</div>

> **"O Control Plane moderno e de alta performance para gerenciar Containers e VMs via LXD"**
>
> Uma alternativa leve e poderosa ao Proxmox, focada em Developer Experience (DX) e interface moderna.

---

## 📖 Introdução

O **Axion** é um Control Plane moderno e de alta performance projetado para gerenciar Containers LXC e Máquinas Virtuais (VMs) via LXD. Inspirado pelas melhores práticas de infraestrutura, o Axion oferece uma experiência de desenvolvedor excepcional com uma interface de usuário elegante e funcionalidades avançadas de monitoramento, backup e governança.

Ao contrário de soluções tradicionais, o Axion foi construído do zero com foco em performance, simplicidade e escalabilidade, proporcionando uma plataforma ágil para ambientes de desenvolvimento, homelabs e até mesmo produções menores.

---

## ✨ Funcionalidades

### 📊 Enterprise Observability
- 📈 **Métricas Históricas**: Coleta de dados de CPU, RAM e disco com retenção configurável
- 🎯 **Gráficos Interativos**: Visualizações em tempo real com opções de período (1h, 24h, 7d, 30d)
- 🕒 **Timeline Completa**: Histórico detalhado de eventos e métricas passadas

### 💾 Backup & Disaster Recovery
- ⏰ **Backups Automatizados**: Sistema completo com agendamento via Cron (@daily, @weekly, etc.)
- 🔄 **Política de Retenção**: Configuração flexível para manter apenas os backups necessários (últimos 7 dias, 90 dias, etc.)
- ♻️ **Rotação de Snapshots**: Limpeza automática de snapshots antigos para economizar espaço

### 🔍 Audit Logs & Timeline
- 👤 **Registro de Eventos**: Acompanhe quem iniciou, parou, criou ou excluiu instâncias
- 📋 **Timeline Detalhada**: Visão cronológica de todas as ações críticas na infraestrutura
- 🕵️ **Auditoria Completa**: Ferramentas para investigar mudanças e incidentes

### 🔁 Auto-Discovery
- 🔄 **Sincronização Inteligente**: Estado automaticamente sincronizado entre LXD e Banco de Dados
- 🎯 **Detecção Automática**: Identificação de novos containers e VMs sem intervenção manual
- ⚡ **Atualização em Tempo Real**: Mantém o dashboard sempre atualizado com o estado real

### 💻 VM Support
- ⚡ **Virtual Machines Full**: Suporte completo a VMs QEMU/KVM além de containers LXC
- 🌐 **Rede Automática**: Configuração de interfaces de rede com IPs e DNS configurados automaticamente
- ☁️ **Cloud-Init Integrado**: Provisionamento inicial de VMs com scripts de inicialização

### 💻 Web Terminal
- 🌐 **Acesso Direto**: Terminal completo via xterm.js integrado no navegador
- ⌨️ **Experiência Nativa**: Funcionalidades completas de terminal dentro do dashboard
- 🔐 **Seguro e Isolado**: Conexão segura via WebSocket com controle granular

### 🚀 Outras Funcionalidades
- 📦 **LXC Containers**: Suporte completo a containers leves e isolados
- 🌐 **Gerenciamento de Rede**: Configuração de bridges, subnets e port forwarding
- 💾 **Storage & Snapshots**: Sistema completo de snapshots e gerenciamento de volumes
- 🔐 **Cluster Mode**: Conexão segura via TLS para múltiplos nós LXD
- ⚙️ **Scheduler Integrado**: Agendamento de tarefas com expressões Cron e persistência
- 📝 **File Explorer**: Gerenciador de arquivos integrado com upload/download
- 💿 **ISO Upload & VM Custom Boot**: Upload de arquivos ISO para instalação personalizada de sistemas operacionais (Windows/Linux)

#### 💿 ISO Upload & VM Custom Boot

O Axion suporta upload de arquivos ISO para criação de VMs com sistemas operacionais personalizados, como Windows ou distribuições Linux que não estejam disponíveis nos repositórios padrão do LXD.

**Funcionalidades principais:**
- Upload de ISOs via interface web com streaming (arquivos grandes não carregam totalmente na RAM)
- Armazenamento seguro no diretório `./data/isos/`
- Criação de VMs vazias configuradas para bootar a partir do ISO
- Configurações específicas para compatibilidade com Windows (secureboot desabilitado)
- Aplicação automática de limites mínimos (2 vCPUs, 4GB RAM)

**Endpoints API:**
- `POST /storage/isos` - Upload de arquivos ISO
- `GET /storage/isos` - Listagem de ISOs disponíveis
- Parâmetro `iso_image` no payload de criação de VM para usar ISO como boot

**Recursos técnicos:**
- Streaming direto para disco sem carregar arquivo completo na memória
- Dispositivos ISO configurados com alta prioridade de boot
- Validação de extensão e proteção contra path traversal
- Integração automática com LXD para configuração de boot ISO

---

## 🏗️ Arquitetura

```
┌─────────────────────┐    ┌──────────────────┐    ┌──────────────────┐
│   Frontend          │    │   Backend        │    │   Infraestrutura │
│   (Next.js)         │◄──►│   (Go API)       │◄──►│   (LXD Socket)   │
│                     │    │                  │    │                  │
│ • React Components  │    │ • Gin Framework  │    │ • LXC Containers │
│ • TailwindCSS UI    │    │ • WebSocket API  │    │ • KVM VMs        │
│ • Charts (Recharts) │    │ • Auth/JWT       │    │ • Network        │
└─────────────────────┘    └──────────────────┘    └──────────────────┘
                                    │
                            ┌──────────────────┐
                            │   Database       │
                            │   (PostgreSQL)   │
                            │                  │
                            │ • Time Series    │
                            │ • Instance Data  │
                            │ • Audit Logs     │
                            └──────────────────┘
```

A arquitetura do Axion segue princípios de separação de responsabilidades, com um frontend moderno em Next.js se comunicando com uma API RESTful em Go, que por sua vez interage com o socket do LXD e gerencia o banco de dados PostgreSQL.

---

## 🚀 Primeiros Passos

### Pré-requisitos

Antes de começar, certifique-se de ter os seguintes componentes instalados:

- [Go](https://golang.org/doc/install) 1.25+
- [Node.js](https://nodejs.org/) 20+ e NPM
- [LXD](https://linuxcontainers.org/lxd/getting-started-cli/) instalado e inicializado
- [PostgreSQL](https://www.postgresql.org/download/) em execução

#### Instalação do LXD e Inicialização
```bash
# Instale o snapd e o LXD (Ubuntu/Debian)
sudo apt update
sudo apt install snapd -y
sudo snap install lxd

# Inicialize o LXD (responda às perguntas conforme sua infraestrutura)
sudo lxd init

# Adicione seu usuário ao grupo lxd (opcional, mas recomendado)
sudo usermod -a -G lxd $USER
```

#### Configuração do PostgreSQL
```bash
# Instale o PostgreSQL
sudo apt install postgresql postgresql-contrib

# Configure um banco de dados e usuário para o Axion
sudo -u postgres psql
CREATE DATABASE axion;
CREATE USER axion_user WITH PASSWORD 'axion_password';
GRANT ALL PRIVILEGES ON DATABASE axion TO axion_user;
\q
```

### Instalação e Execução

#### 1. Clone o Repositório
```bash
git clone https://github.com/pedrortxdev/axion.git
cd axion
```

#### 2. Configurar e Executar o Backend
```bash
# 1. Instale as dependências do Go
go mod tidy

# 2. Configure as variáveis de ambiente (renomeie .env.example para .env)
cp .env.example .env
# Edite .env com suas configurações de banco de dados e LXD

# 3. Inicie o Control Plane (Backend)
go run main.go
```
O Backend iniciará na porta padrão `8500`.

#### 3. Configurar e Executar o Frontend
Em um novo terminal:
```bash
cd frontend

# 1. Instale as dependências
npm install

# 2. Configure o .env (se necessário)
cp .env.example .env
# Configure a URL da API do backend

# 3. Inicie o servidor de desenvolvimento
npm run dev
```
O Dashboard estará acessível em `http://localhost:3500`.

### Estrutura de Desenvolvimento
```
axion/
├── main.go               # Ponto de entrada do backend
├── go.mod/go.sum         # Dependências Go
├── internal/             # Código interno do backend
├── frontend/             # Código Next.js do frontend
├── .env.example          # Exemplo de variáveis de ambiente
└── README.md             # Esta documentação
```

---

## 🧭 Roadmap

### Recursos Futuros Planejados

- 🔧 **Health Checks**: Monitoramento de saúde de instâncias com alertas proativos
- 🛡️ **RBAC (Role-Based Access Control)**: Controle de acesso baseado em funções e permissões
- 📦 **Restore UI**: Interface completa para restauração de backups e snapshots
- 🌐 **Multi-node Clustering**: Suporte a múltiplos nodes LXD com balanceamento
- 📊 **Alerting System**: Sistema de alertas baseado em thresholds de métricas
- 🔐 **SAML/OAuth Integration**: Suporte a provedores de autenticação SSO
- 📈 **Custom Dashboards**: Painéis personalizáveis para diferentes necessidades de monitoramento

---

## 🤝 Contribuindo

Contribuições são bem-vindas! Para contribuir com o Axion:

1. Faça um fork do projeto
2. Crie uma branch para sua feature (`git checkout -b feature/nova-feature`)
3. Commit suas alterações (`git commit -m 'Adiciona nova feature'`)
4. Push para a branch (`git push origin feature/nova-feature`)
5. Abra um Pull Request

---

## 📄 Licença

Este projeto está licenciado sob a Licença MIT - veja o arquivo [LICENSE](LICENSE) para detalhes.

---

## ❤️ Agradecimentos

O Axion é possível graças a:

- [LXD](https://linuxcontainers.org/lxd/) pelo poderoso back-end de containers e VMs
- [Go](https://golang.org/) pela linguagem de programação de alta performance
- [Next.js](https://nextjs.org/) pelo framework web moderno
- [PostgreSQL](https://www.postgresql.org/) pelo banco de dados robusto e confiável

</div>

> **Axion Control Plane** - *Modern Infrastructure Management for the Cloud Era*
