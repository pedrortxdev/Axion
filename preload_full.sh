#!/bin/bash
echo "🚀 Iniciando Download de Imagens (Híbrido: Container + VM)..."

# Função para baixar imagens
# $1 = Remote Source (ex: ubuntu:24.04)
# $2 = Alias Local (ex: ubuntu/24.04)
# $3 = Flag (--vm ou vazio)
download_image() {
    SRC=$1
    ALIAS=$2
    FLAG=$3
    
    echo "------------------------------------------------"
    echo "⬇️  Baixando $SRC para $ALIAS $FLAG..."
    lxc image copy $SRC local: --alias $ALIAS $FLAG --public --auto-update
}

# --- CONTAINERS (LXC) ---
echo "📦 Baixando Containers..."
download_image "ubuntu:24.04" "ubuntu/24.04" ""
download_image "ubuntu:22.04" "ubuntu/22.04" ""

# --- VIRTUAL MACHINES (KVM) ---
echo "🖥️  Baixando VMs (Isso demora mais)..."
# O parametro --vm diz ao LXD para baixar o disco bootável QCOW2/Raw
download_image "ubuntu:24.04" "ubuntu/24.04-vm" "--vm"
download_image "ubuntu:22.04" "ubuntu/22.04-vm" "--vm"

echo "------------------------------------------------"
echo "✅ Concluído! Lista de Imagens Disponíveis:"
lxc image list
