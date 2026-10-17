// status_linux.go reads this computer's battery (UPower) and network
// (NetworkManager) so paired devices can show them.
package main

import (
	"github.com/godbus/dbus/v5"
)

func localStatus() DeviceStatus {
	st := DeviceStatus{Battery: -1, Signal: -1}
	conn, err := dbus.SystemBus()
	if err != nil {
		return st
	}
	bat := conn.Object("org.freedesktop.UPower", "/org/freedesktop/UPower/devices/DisplayDevice")
	if v, err := bat.GetProperty("org.freedesktop.UPower.Device.IsPresent"); err == nil {
		if present, _ := v.Value().(bool); present {
			if p, err := bat.GetProperty("org.freedesktop.UPower.Device.Percentage"); err == nil {
				if f, ok := p.Value().(float64); ok {
					st.Battery = int(f + 0.5)
				}
			}
			if s, err := bat.GetProperty("org.freedesktop.UPower.Device.State"); err == nil {
				state, _ := s.Value().(uint32)
				st.Charging = state == 1 || state == 4 // charging, fully charged
			}
		}
	}

	nm := conn.Object("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager")
	if c, err := nm.GetProperty("org.freedesktop.NetworkManager.Connectivity"); err == nil {
		state, _ := c.Value().(uint32)
		st.Internet = state == 4 // NM_CONNECTIVITY_FULL
	}
	v, err := nm.GetProperty("org.freedesktop.NetworkManager.PrimaryConnection")
	if err != nil {
		return st
	}
	path, _ := v.Value().(dbus.ObjectPath)
	if path == "" || path == "/" {
		return st
	}
	active := conn.Object("org.freedesktop.NetworkManager", path)
	t, _ := active.GetProperty("org.freedesktop.NetworkManager.Connection.Active.Type")
	id, _ := active.GetProperty("org.freedesktop.NetworkManager.Connection.Active.Id")
	kind, _ := t.Value().(string)
	name, _ := id.Value().(string)
	switch kind {
	case "802-11-wireless":
		st.Network, st.SSID = "wifi", name
	case "802-3-ethernet":
		st.Network = "ethernet"
	case "gsm", "cdma":
		st.Network = "cellular"
	}
	return st
}
