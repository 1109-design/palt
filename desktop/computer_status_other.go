//go:build !linux

package main

import "errors"

func screenLocked() bool              { return false }
func addComputerStatus(*DeviceStatus) {}

// WakeStatus is whether phones can wake this computer from sleep.
type WakeStatus struct {
	Ready bool   `json:"ready"`
	Wifi  bool   `json:"wifi"`
	Iface string `json:"iface"`
	Error string `json:"error"`
}

func (a *App) GetWakeStatus() WakeStatus { return WakeStatus{Error: "not supported here"} }
func (a *App) EnableWake() error         { return errors.New("not supported here") }
