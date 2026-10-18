// computer_status_linux.go adds what a phone shows about this computer
// beyond battery and network: locked or not, CPU, memory, disk, uptime,
// temperature, and what it needs to wake this computer from sleep.
package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

// screenLocked asks GNOME whether the screen is locked.
func screenLocked() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	var active bool
	conn.Object("org.gnome.ScreenSaver", "/org/gnome/ScreenSaver").
		Call("org.gnome.ScreenSaver.GetActive", 0).Store(&active)
	return active
}

var cpuPrev struct {
	sync.Mutex
	idle, total uint64
}

// cpuLoad is the CPU use in percent since it was last called.
func cpuLoad() int {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	fields := strings.Fields(strings.SplitN(string(b), "\n", 2)[0])
	var idle, total uint64
	for i, f := range fields[1:] {
		v, _ := strconv.ParseUint(f, 10, 64)
		total += v
		if i == 3 || i == 4 { // idle, iowait
			idle += v
		}
	}
	cpuPrev.Lock()
	defer cpuPrev.Unlock()
	di, dt := idle-cpuPrev.idle, total-cpuPrev.total
	cpuPrev.idle, cpuPrev.total = idle, total
	if dt == 0 || dt < di {
		return 0
	}
	return int(100 * (dt - di) / dt)
}

func meminfo() (used, total int64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	vals := map[string]int64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		n, _ := strconv.ParseInt(strings.Fields(v)[0], 10, 64)
		vals[k] = n * 1024
	}
	return vals["MemTotal"] - vals["MemAvailable"], vals["MemTotal"]
}

// cpuTemp is the CPU package temperature, or the hottest zone.
func cpuTemp() int {
	zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
	best := 0
	for _, z := range zones {
		t, _ := os.ReadFile(filepath.Join(z, "type"))
		v, err := os.ReadFile(filepath.Join(z, "temp"))
		if err != nil {
			continue
		}
		c, _ := strconv.Atoi(strings.TrimSpace(string(v)))
		c /= 1000
		if strings.TrimSpace(string(t)) == "x86_pkg_temp" {
			return c
		}
		if c > best && c < 150 {
			best = c
		}
	}
	return best
}

// wakeInfo is cached: it runs nmcli.
var wakeCache struct {
	sync.Mutex
	at              time.Time
	mac, addr, conn string
	wifi, ready     bool
}

// lanIface is the interface carrying the LAN address.
func lanIface() (*net.Interface, *net.IPNet) {
	ip := net.ParseIP(lanIP())
	ifaces, _ := net.Interfaces()
	for i := range ifaces {
		addrs, _ := ifaces[i].Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.Equal(ip) {
				return &ifaces[i], n
			}
		}
	}
	return nil, nil
}

func refreshWake() {
	wakeCache.Lock()
	defer wakeCache.Unlock()
	if time.Since(wakeCache.at) < 5*time.Minute {
		return
	}
	wakeCache.at = time.Now()
	wakeCache.mac, wakeCache.addr, wakeCache.conn, wakeCache.ready = "", "", "", false
	iface, n := lanIface()
	if iface == nil || n.IP.To4() == nil {
		return
	}
	bcast := make(net.IP, 4)
	for i := range bcast {
		bcast[i] = n.IP.To4()[i] | ^n.Mask[i]
	}
	wakeCache.mac, wakeCache.addr = iface.HardwareAddr.String(), bcast.String()
	out, err := exec.Command("nmcli", "-g", "GENERAL.CONNECTION,GENERAL.TYPE", "device", "show", iface.Name).Output()
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return
	}
	wakeCache.conn, wakeCache.wifi = lines[0], lines[1] == "wifi"
	key := "802-3-ethernet.wake-on-lan"
	if wakeCache.wifi {
		key = "802-11-wireless.wake-on-wlan"
	}
	v, _ := exec.Command("nmcli", "-g", key, "connection", "show", wakeCache.conn).Output()
	wakeCache.ready = wakesOnMagic(strings.TrimSpace(string(v)), wakeCache.wifi)
}

// wakesOnMagic reads NetworkManager's wake flags, shown as names or hex.
func wakesOnMagic(v string, wifi bool) bool {
	if strings.Contains(v, "magic") {
		return true
	}
	n, err := strconv.ParseUint(strings.TrimPrefix(v, "0x"), 16, 32)
	if err != nil {
		return false
	}
	if wifi {
		return n&0x8 != 0 || n&0x2 != 0 // magic, any
	}
	return n&0x40 != 0 // magic
}

func addComputerStatus(st *DeviceStatus) {
	st.Locked = screenLocked()
	st.CPU = cpuLoad()
	st.RAMUsed, st.RAMTotal = meminfo()
	var fs syscall.Statfs_t
	if home, err := os.UserHomeDir(); err == nil && syscall.Statfs(home, &fs) == nil {
		st.DiskFree = int64(fs.Bavail) * fs.Bsize
		st.DiskTotal = int64(fs.Blocks) * fs.Bsize
	}
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		f, _ := strconv.ParseFloat(strings.Fields(string(b))[0], 64)
		st.Uptime = int64(f)
	}
	st.Temp = cpuTemp()
	refreshWake()
	wakeCache.Lock()
	st.WakeMAC, st.WakeAddr, st.WakeReady = wakeCache.mac, wakeCache.addr, wakeCache.ready
	wakeCache.Unlock()
}

// WakeStatus is whether phones can wake this computer from sleep.
type WakeStatus struct {
	Ready bool   `json:"ready"`
	Wifi  bool   `json:"wifi"`
	Iface string `json:"iface"` // the network it would wake on
	Error string `json:"error"`
}

// GetWakeStatus says whether a phone's magic packet wakes this computer.
func (a *App) GetWakeStatus() WakeStatus {
	refreshWake()
	wakeCache.Lock()
	defer wakeCache.Unlock()
	if wakeCache.conn == "" {
		return WakeStatus{Error: "not on a network"}
	}
	return WakeStatus{Ready: wakeCache.ready, Wifi: wakeCache.wifi, Iface: wakeCache.conn}
}

// EnableWake turns on wake-on-magic-packet for the current network
// connection (NetworkManager applies it from the next connect).
func (a *App) EnableWake() error {
	wakeCache.Lock()
	wakeCache.at = time.Time{}
	wakeCache.Unlock()
	refreshWake()
	wakeCache.Lock()
	conn, wifi := wakeCache.conn, wakeCache.wifi
	wakeCache.at = time.Time{}
	wakeCache.Unlock()
	if conn == "" {
		return fmt.Errorf("not on a network")
	}
	key := "802-3-ethernet.wake-on-lan"
	if wifi {
		key = "802-11-wireless.wake-on-wlan"
	}
	if out, err := exec.Command("nmcli", "connection", "modify", conn, key, "magic").CombinedOutput(); err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	// Apply it now without dropping the connection where NetworkManager can.
	iface, _ := lanIface()
	if iface != nil {
		exec.Command("nmcli", "device", "reapply", iface.Name).Run()
	}
	return nil
}
