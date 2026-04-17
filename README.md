<p align="center">
  <img src="icon.png" alt="spank logo" width="200">
</p>

# Spank Control Center

[English](README-en.md) | **[Français](README.md)**

Une application macOS complètement native et ultra-fluide qui détecte les claques (slaps) données à votre MacBook et y répond par des effets sonores ! 

Spank Control Center embarque une interface visuelle dédiée (Webview Native Apple) vous permettant de gérer vos propres banques de sons ou d'utiliser les nombreux packs intégrés.

> **Note :** Ce projet repensé avec une interface graphique complète et packagé en application macOS de bureau est basé sur un fork du projet CLI original (`taigrr/spank`).

## 🌟 Fonctionnalités uniques

- **Application macOS Autonome :** Fini les lignes de commande complexes. Lancez **`Spank.app`** et gérez tout depuis l'interface visuelle. La fenêtre se comporte comme n'importe quel logiciel Mac, avec son icône dans le dock.
- **Créateur de Modes Personnalisés :** Glissez-déposez n'importe quels fichiers de musiques ou de sons `.mp3` dans la zone de dépôt, donnez-lui un nom, et c'est tout ! L'application renomme et sauvegarde tout intelligemment.
- **Détection temps réel (Apple Silicon) :** Utilise l'accéléromètre natif des puces M1, M2, M3 etc. pour mesurer précisément quand l'ordinateur encaisse un choc.
- **Contrôle en 1 clic :** Naviguez entre vos différents packs (ex: lancer le thème *Sexy* ou le thème *Donkey*), supprimez vos créations, lancez la détection ("*Play*") ou coupez la ("*Stop*") de manière totalement dynamique.

## 🚀 Comment l'utiliser (Installation)

L'installation ne cible **exclusivement** que les puces **Apple Silicon** (M1 et versions supérieures).  

1. Rendez-vous dans la racine du projet ou dans les **Releases** pour télécharger le fichier **`Spank.dmg`**.
2. Double-cliquez sur `Spank.dmg` et glissez simplement **Spank.app** dans votre dossier *Applications*.
3. Ouvrez Spank (macOS vous demandera votre mot de passe, c'est indispensable pour accorder le droit de lire les capteurs physiques de l'accéléromètre !).

## 🛠️ Pour les Développeurs (Modifier et Packager l'App)

L'architecture est entièrement contenue dans le fichier `build_mac_app.sh`. Si vous modifiez le code (HTML du control center, logique du serveur Go, ou le wrapper Swift WKWebView), vous devez re-compiler l'application système.

**📋 Prérequis de build :**
- **MacOS Apple Silicon** (M1/M2/M3).
- **Golang** installé sur le système.
- Les **Command Line Tools Xcode** d'Apple (installés par défaut sur de nombreux Mac, ils fournissent le programme `swiftc` ou `sips` pour les icônes).
- Le packet **create-dmg** via Homebrew (`brew install create-dmg`).

```bash
# 1. Compile le binaire Go, les icones macOS, le wrapper natif Swift WKWebView, et génère "Spank.app"
./build_mac_app.sh

# 2. Package l'application fraîchement compilée dans un DMG super-visuel pour la distribution :
rm -rf dmg_source Spank.dmg
mkdir dmg_source
mv Spank.app Install_Spank.command dmg_source/

create-dmg \
  --volname "Spank App Installer" \
  --window-pos 200 120 \
  --window-size 600 400 \
  --icon-size 100 \
  --icon "Spank.app" 150 150 \
  --app-drop-link 450 150 \
  --icon "Install_Spank.command" 300 250 \
  "Spank.dmg" \
  "dmg_source/"
```

## 🤔 Comment ça marche en coulisses ?

- **Interface Mac Native :** `Spank.app` n'est pas un site Safari classique. Le build script va compiler via `swiftc` une capsule Cocoa Native de macOS qui propulse une vue WKWebView en plein code.
- **La boucle Serveur Go :** Derrière cette magnifique interface se cache l'âme du projet : un binaire Go en administrateur doté de multi-threading (Goroutines). C'est lui qui lit toutes les commandes API silencieusement et donne vie au son !

---
*Mentions : Ce projet exploite les algorithmes de détection d'accéléromètre Apple Silicon originellement portés par [olvvier/apple-silicon-accelerometer](https://github.com/olvvier/apple-silicon-accelerometer) et est un fork UI-Native de l'utilitaire CLI [taigrr/spank](https://github.com/taigrr/spank).*
