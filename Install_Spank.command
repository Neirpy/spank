#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
APP_PATH="$DIR/Spank.app"

echo "=========================================="
echo "🍑 Spank Control Center Installer"
echo "=========================================="

if [ -d "$APP_PATH" ]; then
    echo "1. Déblocage des sécurités Apple Gatekeeper..."
    xattr -cr "$APP_PATH" 2>/dev/null
    
    echo "2. Copie de l'application vers /Applications..."
    cp -R "$APP_PATH" /Applications/
    
    echo "3. Lancement de Spank !"
    open /Applications/Spank.app
    
    echo "✅ Installation terminée avec succès."
    echo "Vous pouvez fermer cette fenêtre."
else
    echo "❌ Erreur : Spank.app introuvable à côté du script."
fi
exit 0
