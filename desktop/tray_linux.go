// tray_linux.go puts PALT in the system tray and starts it on login.
//
// The tray speaks the StatusNotifierItem DBus protocol (KDE, GNOME with the
// AppIndicator extension, XFCE, Cinnamon, ...), so it runs beside the Wails
// GTK loop without needing one of its own. When no tray host is running the
// window behaves as before: closing it quits PALT.
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"fyne.io/systray"
	"github.com/godbus/dbus/v5"
	"github.com/wailsapp/wails/v2/pkg/options"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"palt/internal/models"
	"palt/internal/transfer"
)

//go:embed trayicon.png
var trayIcon []byte

// trayState is the tray menu as last built, so peer updates that change
// nothing visible don't rebuild it.
type trayState struct {
	mu        sync.Mutex
	ready     bool
	signature string
	end       func()
}

// trayAvailable reports whether a tray host is listening on the session bus.
func trayAvailable() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	var owned bool
	err = conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, "org.kde.StatusNotifierWatcher").Store(&owned)
	return err == nil && owned
}

// signalRunningInstance hands this launch to an instance that is already
// running and reports whether there was one. Wails has the same check, but
// runs OnStartup alongside it, so a second copy would die on the busy port
// before it got the chance.
func signalRunningInstance(uniqueID string) bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	// Name and path as Wails registers them for SingleInstanceLock.
	id := "wails_app_" + strings.NewReplacer("-", "_", ".", "_").Replace(uniqueID)
	name := "org." + id + ".SingleInstance"
	var owned bool
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, name).Store(&owned); err != nil || !owned {
		return false
	}
	data := options.SecondInstanceData{Args: os.Args[1:]}
	data.WorkingDirectory, _ = os.Getwd()
	msg, err := json.Marshal(data)
	if err != nil {
		return false
	}
	return conn.Object(name, dbus.ObjectPath("/org/"+id+"/SingleInstance")).Call(name+".SendMessage", 0, string(msg)).Err == nil
}

// startTray shows the tray icon. Call it once the Wails context exists.
func (a *App) startTray() {
	if !a.trayEnabled {
		return
	}
	start, end := systray.RunWithExternalLoop(func() {
		systray.SetIcon(trayIcon)
		systray.SetTitle("PALT")
		systray.SetOnTapped(a.showWindow)
		a.tray.mu.Lock()
		a.tray.ready = true
		a.tray.mu.Unlock()
		a.refreshTray()
	}, nil)
	a.tray.end = end
	start()
}

func (a *App) stopTray() {
	if a.tray.end != nil {
		a.tray.end()
	}
}

// refreshTray rebuilds the menu when the device list or its names change.
func (a *App) refreshTray() {
	if !a.trayEnabled {
		return
	}
	peers := a.mergedPeers()
	aliases := a.GetAliases()
	sort.Slice(peers, func(i, j int) bool { return peers[i].DeviceName < peers[j].DeviceName })

	labels := make([]string, len(peers))
	for i, p := range peers {
		labels[i] = trayPeerLabel(p, aliases[p.DeviceName], a.statusOf(p.DeviceName))
	}
	autostart := autostartEnabled()
	paired := make([]bool, len(peers))
	for i, p := range peers {
		paired[i] = p.Paired
	}
	signature := fmt.Sprintf("%q %v %v", labels, paired, autostart)

	a.tray.mu.Lock()
	defer a.tray.mu.Unlock()
	if !a.tray.ready || signature == a.tray.signature {
		return
	}
	a.tray.signature = signature

	systray.ResetMenu()
	switch len(peers) {
	case 0:
		systray.SetTooltip("PALT — no devices connected")
	case 1:
		systray.SetTooltip("PALT — 1 device connected")
	default:
		systray.SetTooltip(fmt.Sprintf("PALT — %d devices connected", len(peers)))
	}

	header := systray.AddMenuItem("Connected devices", "")
	header.Disable()
	if len(peers) == 0 {
		none := systray.AddMenuItem("    None nearby", "")
		none.Disable()
	}
	for i, p := range peers {
		device := systray.AddMenuItem(labels[i], "")
		a.trayDeviceMenu(device, p)
	}

	systray.AddSeparator()
	open := systray.AddMenuItem("Open PALT", "")
	login := systray.AddMenuItemCheckbox("Start on login", "", autostart)
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit", "")
	go func() {
		for range open.ClickedCh {
			a.showWindow()
		}
	}()
	go func() {
		for range login.ClickedCh {
			if err := setAutostart(!autostartEnabled()); err != nil {
				log.Printf("[tray] Could not change start on login: %v", err)
			}
			a.refreshTray()
		}
	}()
	go func() {
		for range quit.ClickedCh {
			a.quit()
		}
	}()
}

// trayDeviceMenu fills a device's submenu.
func (a *App) trayDeviceMenu(device *systray.MenuItem, p models.Peer) {
	name := p.DeviceName
	on := func(label string, fn func() error) {
		item := device.AddSubMenuItem(label, "")
		go func() {
			for range item.ClickedCh {
				if err := fn(); err != nil {
					notify(Notification{Title: "PALT", Body: err.Error(), Icon: "dialog-warning"})
				}
			}
		}()
	}
	on("Open chat", func() error {
		a.showWindow()
		wailsruntime.EventsEmit(a.ctx, "tray_open_device", name)
		return nil
	})
	on("Send files…", func() error {
		a.showWindow()
		return a.SendFile(p.IPAddress, p.Port, name)
	})
	on("Send clipboard", func() error { return a.SendClipboard(name) })
	if p.Paired {
		on("Ring", func() error { return a.RingDevice(name) })
		on("Ping", func() error { return a.PingDevice(name) })
	}
}

func trayPeerLabel(p models.Peer, alias string, st DeviceStatus) string {
	name := p.DeviceName
	if alias != "" {
		name = alias
	}
	if st.Battery >= 0 {
		name += fmt.Sprintf("  · %d%%", st.Battery)
		if st.Charging {
			name += " ⚡"
		}
	}
	switch st.Network {
	case "wifi":
		if st.SSID != "" {
			name += "  · " + st.SSID
		}
	case "cellular":
		name += "  · mobile data"
	}
	if p.Via == transfer.RouteRelay {
		name += "  · relay"
	}
	return "    " + name
}

// ─── Start on login ──────────────────────────────────────────────────────────

// startHiddenFlag is passed by the login entry so PALT starts in the tray.
const startHiddenFlag = "--hidden"

func autostartFile() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "autostart", "palt.desktop"), nil
}

// autostartOffMarker records that the user turned start on login off, so it
// isn't switched back on at the next launch.
func autostartOffMarker() string {
	dir, err := configDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "autostart_off")
}

func autostartEnabled() bool {
	file, err := autostartFile()
	if err != nil {
		return false
	}
	_, err = os.Stat(file)
	return err == nil
}

func setAutostart(enabled bool) error {
	file, err := autostartFile()
	if err != nil {
		return err
	}
	marker := autostartOffMarker()
	if !enabled {
		if marker != "" {
			_ = os.WriteFile(marker, nil, 0o644)
		}
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if marker != "" {
		_ = os.Remove(marker)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	entry := strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=PALT",
		"Comment=Peer And Local Transfer",
		"Exec=" + exe + " " + startHiddenFlag,
		"Icon=palt",
		"Terminal=false",
		"X-GNOME-Autostart-enabled=true",
		"",
	}, "\n")
	return os.WriteFile(file, []byte(entry), 0o644)
}

// ensureAutostart turns start on login on by default, and refreshes the entry
// so it follows the binary if it moved. A user's opt-out is respected, and
// development builds are left out.
func ensureAutostart() {
	if exe, err := os.Executable(); err != nil || strings.Contains(exe, "/build/bin/") {
		return
	}
	if marker := autostartOffMarker(); marker != "" {
		if _, err := os.Stat(marker); err == nil {
			return
		}
	}
	if err := setAutostart(true); err != nil {
		log.Printf("[tray] Could not set up start on login: %v", err)
	}
}
