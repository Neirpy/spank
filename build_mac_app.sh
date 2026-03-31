#!/bin/bash

# Configuration
APP_NAME="Spank"
APP_BUNDLE="${APP_NAME}.app"

echo "🔨 Compilation du binaire Go..."
CGO_ENABLED=0 go build -o spank .

if [ ! -f "spank" ]; then
    echo "❌ Erreur de compilation"
    exit 1
fi

echo "🍏 Création du lanceur AppleScript..."
cat << 'EOF' > launcher.applescript
set actionList to {"Démarrer - Donkey (Défaut)", "Démarrer - SCo", "Démarrer - Pain", "Démarrer - Sexy", "Démarrer - Halo"}
set chosenAction to choose from list actionList with prompt "Spank : Choisissez le mode audio" default items {"Démarrer - Donkey (Défaut)"} with title "Spank Launcher"

if chosenAction is not false then
	set action to item 1 of chosenAction
	
	set spankBin to POSIX path of (path to resource "spank")
	set spankMode to "--donkey"
	
	if action is "Démarrer - SCo" then
		set spankMode to "--SC"
	else if action is "Démarrer - Pain" then
		set spankMode to "--pain"
	else if action is "Démarrer - Sexy" then
		set spankMode to "--sexy"
	else if action is "Démarrer - Halo" then
		set spankMode to "--halo"
	end if
	
	set runCmd to "sudo killall spank 2>/dev/null; sudo '" & spankBin & "' " & spankMode
	
	tell application "Terminal"
		activate
		do script runCmd
	end tell
end if
EOF

echo "📦 Packaging de la vraie application Mac (.app)..."
osacompile -o "${APP_BUNDLE}" launcher.applescript

cp spank "${APP_BUNDLE}/Contents/Resources/"

rm launcher.applescript spank

echo "✅ Terminé ! Tu as maintenant une vraie application '${APP_BUNDLE}'."
echo "👉 Tu peux maintenant tester en double-cliquant sur ${APP_BUNDLE}"
echo "👉 Tu pourras ensuite glisser-déposer ${APP_BUNDLE} dans ton dossier /Applications"
