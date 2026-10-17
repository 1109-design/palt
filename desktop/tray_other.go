//go:build !linux

// tray_other.go: the tray and start on login are Linux-only for now.
package main

const startHiddenFlag = "--hidden"

type trayState struct{}

func trayAvailable() bool { return false }

func signalRunningInstance(string) bool { return false }

func ensureAutostart() {}

func (a *App) startTray() {}

func (a *App) stopTray() {}

func (a *App) refreshTray() {}
