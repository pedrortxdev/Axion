# AxHV Deployment Guide

## Systemd Service (Production)

Para rodar o AxHV Daemon como um serviço de sistema gerenciado (o "processo da axion"):

### 1. Instalação do Binário

Copie o binário compilado para `/usr/local/bin`:

```bash
# Assumindo que você está na raiz do projeto
# Nota: Ajuste 'target/debug' para 'target/release' se estiver usando build de produção
sudo cp target/debug/axhv-daemon /usr/local/bin/axhv-daemon
sudo chmod +x /usr/local/bin/axhv-daemon
```

### 2. Configuração do Serviço

Crie o arquivo `/etc/systemd/system/axhv-daemon.service` com o seguinte conteúdo:

```ini
[Unit]
Description=AxHV Daemon - High Performance MicroVM Hypervisor
Documentation=https://github.com/pedrortxdev/AxHV-v2
After=network.target

[Service]
Type=simple
User=root
Group=root
# Caminho para o binário instalado
ExecStart=/usr/local/bin/axhv-daemon
# Diretório de trabalho para arquivos de estado (vms.json)
WorkingDirectory=/var/lib/axhv
# Reiniciar automaticamente em caso de falha
Restart=always
RestartSec=3
# Configuração de Logs e Ambiente
Environment=RUST_LOG=info
Environment=AXHV_CLUSTER_TOKEN=dev-token

[Install]
WantedBy=multi-user.target
```

**Nota:** Certifique-se de que o diretório de trabalho existe:
```bash
sudo mkdir -p /var/lib/axhv
```

### 3. Ativação do Serviço

Recarregue o daemon do systemd, habilite o serviço para iniciar no boot e inicie-o imediatamente:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now axhv-daemon
```

### 4. Gerenciamento

Verificar status e logs:
```bash
sudo systemctl status axhv-daemon
# Ver logs em tempo real
sudo journalctl -u axhv-daemon -f
```

Parar ou Reiniciar:
```bash
sudo systemctl stop axhv-daemon
sudo systemctl restart axhv-daemon
```
