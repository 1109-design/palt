// wifi_linux.go reads and joins Wi-Fi networks through NetworkManager's
// nmcli, which asks the session's secret agent for saved passwords.
package main

import (
	"fmt"
	"os/exec"
	"strings"

	"palt/internal/wifiqr"
)

// currentWifi returns the Wi-Fi network this computer is on, with its
// password.
func currentWifi() (wifiqr.Network, error) {
	out, err := exec.Command("nmcli", "-t", "-f", "NAME,TYPE", "connection", "show", "--active").Output()
	if err != nil {
		return wifiqr.Network{}, fmt.Errorf("NetworkManager isn't available")
	}
	conn := ""
	for _, line := range strings.Split(string(out), "\n") {
		if name, ok := strings.CutSuffix(line, ":802-11-wireless"); ok {
			conn = strings.ReplaceAll(name, `\:`, ":")
			break
		}
	}
	if conn == "" {
		return wifiqr.Network{}, fmt.Errorf("this computer isn't on Wi-Fi")
	}
	get := func(field string) string {
		out, _ := exec.Command("nmcli", "-s", "-g", field, "connection", "show", "id", conn).Output()
		return strings.TrimRight(string(out), "\n")
	}
	n := wifiqr.Network{SSID: get("802-11-wireless.ssid"), Hidden: get("802-11-wireless.hidden") == "yes"}
	switch get("802-11-wireless-security.key-mgmt") {
	case "":
		n.Security = "nopass"
	case "none":
		n.Security = "WEP"
		n.Password = get("802-11-wireless-security.wep-key0")
	case "wpa-psk", "sae":
		n.Security = "WPA"
		n.Password = get("802-11-wireless-security.psk")
	default:
		return wifiqr.Network{}, fmt.Errorf("%s uses enterprise sign-in, which can't be shared", n.SSID)
	}
	if n.SSID == "" || (n.Security != "nopass" && n.Password == "") {
		return wifiqr.Network{}, fmt.Errorf("couldn't read the password for %s", conn)
	}
	return n, nil
}

// joinWifi connects to a network, saving it for next time.
func joinWifi(n wifiqr.Network) error {
	args := []string{"device", "wifi", "connect", n.SSID}
	if n.Security != "nopass" && n.Password != "" {
		args = append(args, "password", n.Password)
	}
	if n.Hidden {
		args = append(args, "hidden", "yes")
	}
	out, err := exec.Command("nmcli", args...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		msg = strings.TrimPrefix(msg, "Error: ")
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("couldn't join %s: %s", n.SSID, msg)
	}
	return nil
}
