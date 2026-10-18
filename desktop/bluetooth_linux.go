package main

import (
	"strings"

	"github.com/godbus/dbus/v5"
)

// localBluetooth reads the adapter and connected devices from BlueZ.
func localBluetooth() BluetoothState {
	st := BluetoothState{Devices: []BluetoothDevice{}}
	conn, err := dbus.SystemBus()
	if err != nil {
		return st
	}
	var objs map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if conn.Object("org.bluez", "/").Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objs) != nil {
		return st
	}
	for _, ifs := range objs {
		if ad, ok := ifs["org.bluez.Adapter1"]; ok {
			st.Available = true
			if v, ok := ad["Powered"].Value().(bool); ok && v {
				st.On = true
			}
		}
		dev, ok := ifs["org.bluez.Device1"]
		if !ok {
			continue
		}
		if c, _ := dev["Connected"].Value().(bool); !c {
			continue
		}
		d := BluetoothDevice{Battery: -1}
		d.Name, _ = dev["Alias"].Value().(string)
		if d.Name == "" {
			d.Name, _ = dev["Name"].Value().(string)
		}
		icon, _ := dev["Icon"].Value().(string)
		d.Kind = strings.TrimPrefix(strings.TrimPrefix(icon, "audio-"), "input-")
		if b, ok := ifs["org.bluez.Battery1"]; ok {
			if pct, ok := b["Percentage"].Value().(byte); ok {
				d.Battery = int(pct)
			}
		}
		st.Devices = append(st.Devices, d)
	}
	return st
}

// setLocalBluetooth turns Bluetooth on or off the way GNOME's quick settings
// do (rfkill through the settings daemon), then powers the adapter.
func setLocalBluetooth(on bool) error {
	if sess, err := dbus.SessionBus(); err == nil {
		sess.Object("org.gnome.SettingsDaemon.Rfkill", "/org/gnome/SettingsDaemon/Rfkill").
			SetProperty("org.gnome.SettingsDaemon.Rfkill.BluetoothAirplaneMode", dbus.MakeVariant(!on))
	}
	sys, err := dbus.SystemBus()
	if err != nil {
		return err
	}
	var objs map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := sys.Object("org.bluez", "/").Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objs); err != nil {
		if !on {
			return nil // BlueZ gone with the adapter: it's off
		}
		return err
	}
	var last error
	for path, ifs := range objs {
		if _, ok := ifs["org.bluez.Adapter1"]; ok {
			if err := sys.Object("org.bluez", path).SetProperty("org.bluez.Adapter1.Powered", dbus.MakeVariant(on)); err != nil {
				last = err
			}
		}
	}
	return last
}
