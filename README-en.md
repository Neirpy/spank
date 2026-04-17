<p align="center">
  <img src="icon.png" alt="spank logo" width="200">
</p>

# Spank Control Center

**[English](README-en.md)** | [Français](README.md)

A fully native and ultra-smooth macOS application that detects physical slaps on your MacBook and responds with sound effects!

Spank Control Center features a dedicated visual interface (Native Apple Webview) allowing you to manage your own custom sound banks or use multiple built-in packs.

> **Note:** This redesigned project with a complete GUI and packaged as a desktop macOS application is based on a fork of the original CLI project (`taigrr/spank`).

## 🌟 Unique Features

- **Standalone macOS Application:** No more complex command lines. Launch **`Spank.app`** and manage everything from the visual interface. The window behaves like any Mac software, with its own independent icon in the dock.
- **Custom Mode Creator:** Drag and drop any `.mp3` music or sound files into the drop zone, give it a name, and that's it! The application smartly renames and saves everything.
- **Real-time Detection (Apple Silicon):** Uses the native accelerometer of M1, M2, M3 chips to precisely measure when the computer takes an impact.
- **1-Click Control:** Switch between your different packs (e.g. launch the *Sexy* theme or the *Donkey* theme), delete your custom creations, start the detection ("*Play*"), or stop it ("*Stop*") dynamically.

## 🚀 How to Use It (Installation)

The installation **exclusively** targets **Apple Silicon** chips (M1 and higher versions).

1. Go to the project root or the **Releases** section to download the **`Spank.dmg`** file.
2. Double-click on `Spank.dmg` and simply drag **Spank.app** into your *Applications* folder.
3. Open Spank (macOS will ask for your administrator password, this is mandatory to grant the right to read physical sensors from the accelerometer!).

## 🛠️ For Developers (Modify & Package the App)

The architecture is entirely contained within the `build_mac_app.sh` file. If you modify the code (HTML of the control center, Go server logic, or the WKWebView Swift wrapper), you must re-compile the system application.

**📋 Build Prerequisites:**
- **MacOS Apple Silicon** (M1/M2/M3).
- **Golang** installed on your system.
- Apple's **Xcode Command Line Tools** (installed by default on many Macs, providing systems utilities like `swiftc` or `sips` for icons).
- The **create-dmg** package via Homebrew (`brew install create-dmg`).

```bash
# 1. Compiles the Go binary, macOS icons, the native Swift WKWebView wrapper, and generates "Spank.app"
./build_mac_app.sh

# 2. Packages the freshly compiled application into a visually-stunning DMG for distribution:
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

## 🤔 How it works under the hood

- **Native Mac Interface:** `Spank.app` is not a traditional Safari site. The build script uses `swiftc` to compile a native macOS Cocoa capsule that drives a WKWebView entirely in code.
- **The Go Server Loop:** Behind this beautiful interface hides the soul of the project: a multi-threaded Go binary (Goroutines) running as administrator. It reads all API commands silently and brings the sound to life!

---
*Credits: This project utilizes the Apple Silicon accelerometer detection algorithms originally ported by [olvvier/apple-silicon-accelerometer](https://github.com/olvvier/apple-silicon-accelerometer) and is a UI-Native fork of the CLI utility [taigrr/spank](https://github.com/taigrr/spank).*
