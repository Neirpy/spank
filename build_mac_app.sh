#!/bin/bash
set -e

# Configuration
APP_NAME="Spank"
APP_BUNDLE="${APP_NAME}.app"

echo "🔨 Compilation du binaire Go..."
CGO_ENABLED=0 go build -o spank .

if [ ! -f "spank" ]; then
    echo "❌ Erreur de compilation"
    exit 1
fi

echo "🎨 Création de l'icône .icns à partir de icon.png..."
mkdir -p Spank.iconset
sips -z 16 16     icon.png --out Spank.iconset/icon_16x16.png > /dev/null
sips -z 32 32     icon.png --out Spank.iconset/icon_16x16@2x.png > /dev/null
sips -z 32 32     icon.png --out Spank.iconset/icon_32x32.png > /dev/null
sips -z 64 64     icon.png --out Spank.iconset/icon_32x32@2x.png > /dev/null
sips -z 128 128   icon.png --out Spank.iconset/icon_128x128.png > /dev/null
sips -z 256 256   icon.png --out Spank.iconset/icon_128x128@2x.png > /dev/null
sips -z 256 256   icon.png --out Spank.iconset/icon_256x256.png > /dev/null
sips -z 512 512   icon.png --out Spank.iconset/icon_256x256@2x.png > /dev/null
sips -z 512 512   icon.png --out Spank.iconset/icon_512x512.png > /dev/null
sips -z 1024 1024 icon.png --out Spank.iconset/icon_512x512@2x.png > /dev/null
iconutil -c icns Spank.iconset -o AppIcon.icns
rm -R Spank.iconset

echo "🚀 Construction du Wrapper UI Swift..."
cat << 'EOF' > webview.swift
import Cocoa
import WebKit

class AppDelegate: NSObject, NSApplicationDelegate, WKUIDelegate {
    var window: NSWindow!
    
    func applicationDidFinishLaunching(_ aNotification: Notification) {
        let appName = "Spank"
        let app = NSApplication.shared
        let mainMenu = NSMenu()
        let appMenuItem = NSMenuItem()
        mainMenu.addItem(appMenuItem)
        app.mainMenu = mainMenu
        
        let appMenu = NSMenu()
        let quitMenuItem = NSMenuItem(title: "Quit \(appName)", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        appMenu.addItem(quitMenuItem)
        appMenuItem.submenu = appMenu

        window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 900, height: 750),
                          styleMask: [.titled, .closable, .miniaturizable, .resizable],
                          backing: .buffered, defer: false)
        window.center()
        window.title = "Spank Control Center"
        
        let webView = WKWebView(frame: window.contentView!.bounds)
        webView.autoresizingMask = [.width, .height]
        webView.uiDelegate = self
        webView.setValue(false, forKey: "drawsBackground")
        window.contentView?.addSubview(webView)
        
        let url = URL(string: "http://localhost:8080")!
        
        window.makeKeyAndOrderFront(nil)
        
        let task = Process()
        task.launchPath = "/usr/bin/osascript"
        let resourcePath = Bundle.main.resourcePath!
        task.arguments = ["-e", "do shell script \"sudo killall spank 2>/dev/null; '\(resourcePath)/spank' --ui > /tmp/spank_ui.log 2>&1 &\" with administrator privileges"]
        task.launch()
        task.waitUntilExit()
        
        usleep(500000)
        webView.load(URLRequest(url: url))
    }
    
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        return true
    }
    
    func applicationWillTerminate(_ aNotification: Notification) {
        var req = URLRequest(url: URL(string: "http://localhost:8080/api/quit")!)
        req.httpMethod = "POST"
        let group = DispatchGroup()
        group.enter()
        URLSession.shared.dataTask(with: req) { _,_,_ in group.leave() }.resume()
        _ = group.wait(timeout: .now() + 1.0)
    }

    func webView(_ webView: WKWebView, runJavaScriptConfirmPanelWithMessage message: String, initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping (Bool) -> Void) {
        let alert = NSAlert()
        alert.messageText = message
        alert.addButton(withTitle: "OK")
        alert.addButton(withTitle: "Cancel")
        let response = alert.runModal()
        completionHandler(response == .alertFirstButtonReturn)
    }

    func webView(_ webView: WKWebView, runJavaScriptAlertPanelWithMessage message: String, initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping () -> Void) {
        let alert = NSAlert()
        alert.messageText = message
        alert.addButton(withTitle: "OK")
        alert.runModal()
        completionHandler()
    }
}

let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.setActivationPolicy(.regular)
app.run()
EOF

swiftc webview.swift -o SpankLauncher

echo "📦 Packaging de la vraie application Mac (.app)..."
rm -rf "${APP_BUNDLE}"
mkdir -p "${APP_BUNDLE}/Contents/MacOS"
mkdir -p "${APP_BUNDLE}/Contents/Resources"

# Placement des binaires
mv SpankLauncher "${APP_BUNDLE}/Contents/MacOS/Spank"
cp spank "${APP_BUNDLE}/Contents/Resources/"
mv AppIcon.icns "${APP_BUNDLE}/Contents/Resources/"

# Création du Info.plist
cat << EOF > "${APP_BUNDLE}/Contents/Info.plist"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>Spank</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>CFBundleIdentifier</key>
    <string>com.taigrr.spank</string>
    <key>CFBundleName</key>
    <string>Spank</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0</string>
    <key>LSMinimumSystemVersion</key>
    <string>12.0</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
EOF

echo "🔐 Signature de l'application..."
codesign --force --deep -s - "${APP_BUNDLE}"

echo "📜 Création du script d'installation de contournement..."
cat << 'EOF' > Install_Spank.command
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
EOF
chmod +x Install_Spank.command

rm webview.swift spank
echo "✅ Terminé ! Tu as maintenant une vraie application '${APP_BUNDLE}'."
echo "👉 Pour la distribuer, crée un dossier contenant Spank.app et Install_Spank.command, puis build ton DMG dessus !"
