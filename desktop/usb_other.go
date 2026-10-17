//go:build !linux

package main

func isUSBAddr(string) bool { return false }

func (a *App) watchUSBTether() {}
