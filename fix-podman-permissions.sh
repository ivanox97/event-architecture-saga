#!/bin/bash
# Script para arreglar permisos de Podman

echo "🔧 Arreglando permisos de Podman..."
echo ""
echo "El directorio ~/.config pertenece a root, necesitamos cambiarlo:"
echo ""

if [ -d ~/.config ] && [ "$(stat -f '%Su' ~/.config)" != "$USER" ]; then
    echo "⚠️  ~/.config pertenece a: $(stat -f '%Su' ~/.config)"
    echo ""
    echo "Ejecutando: sudo chown -R $USER:$(id -gn) ~/.config"
    sudo chown -R "$USER:$(id -gn)" ~/.config
    echo ""
    if [ "$(stat -f '%Su' ~/.config)" == "$USER" ]; then
        echo "✅ Permisos arreglados exitosamente!"
        echo ""
        echo "Ahora puedes ejecutar: make podman-start"
    else
        echo "❌ Error al arreglar permisos"
        exit 1
    fi
else
    echo "✅ Los permisos de ~/.config ya están correctos"
fi

