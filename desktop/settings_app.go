// settings_app.go keeps per-device feature switches ("show this phone's
// notifications", "sync clipboard", ...), keyed by device name.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Defaults for every switch; a device without saved settings gets these.
var deviceSettingDefaults = map[string]bool{
	"notifications":    true, // show the device's notifications here
	"clipboardSend":    true, // send this computer's clipboard to it
	"clipboardReceive": true, // take its clipboard
	"pauseOnCall":      true, // pause media when a call comes in
}

var settingsMu sync.Mutex

func deviceSettingsFile() string {
	dir, err := configDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "device_settings.json")
}

func loadDeviceSettings() map[string]map[string]bool {
	all := map[string]map[string]bool{}
	if f := deviceSettingsFile(); f != "" {
		if data, err := os.ReadFile(f); err == nil {
			_ = json.Unmarshal(data, &all)
		}
	}
	return all
}

// setting reports a device's switch, falling back to the default.
func (a *App) setting(device, key string) bool {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	if v, ok := loadDeviceSettings()[device][key]; ok {
		return v
	}
	return deviceSettingDefaults[key]
}

// GetDeviceSettings returns every switch for a device.
func (a *App) GetDeviceSettings(device string) map[string]bool {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	out := map[string]bool{}
	for k, v := range deviceSettingDefaults {
		out[k] = v
	}
	for k, v := range loadDeviceSettings()[device] {
		out[k] = v
	}
	return out
}

// SetDeviceSetting changes one switch for a device.
func (a *App) SetDeviceSetting(device, key string, on bool) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	all := loadDeviceSettings()
	if all[device] == nil {
		all[device] = map[string]bool{}
	}
	all[device][key] = on
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(deviceSettingsFile(), data, 0o644)
}
