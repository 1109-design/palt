// cli_app.go handles command-line requests, whether PALT was started with
// them or they came from a second launch (forwarded by the single-instance
// lock). The Nautilus "Send to" menu uses this.
//
//	palt --send <device> <file>...   send files to a device (name, alias or ID)
//	palt --ring <device>             ring a paired device
//	palt --hidden                    start in the tray
package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"
)

// findPeerName resolves a device given as name, alias or device ID.
func (a *App) findPeerName(dev string) (string, bool) {
	aliases := a.GetAliases()
	for _, p := range a.mergedPeers() {
		if p.DeviceName == dev || p.DeviceID == dev || strings.EqualFold(aliases[p.DeviceName], dev) {
			return p.DeviceName, true
		}
	}
	return "", false
}

// handleArgs runs the command in args. A device that isn't connected yet
// gets a little time to show up, so "palt --send" works right after login.
func (a *App) handleArgs(args []string, workDir string) {
	if len(args) < 2 || (args[0] != "--send" && args[0] != "--ring") {
		return
	}
	cmd, dev := args[0], args[1]
	go func() {
		var name string
		for i := 0; i < 40; i++ {
			if n, ok := a.findPeerName(dev); ok {
				name = n
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		if name == "" {
			a.cliError(fmt.Errorf("%s is not connected", dev))
			return
		}
		switch cmd {
		case "--ring":
			a.cliError(a.RingDevice(name))
		case "--send":
			var paths []string
			for _, p := range args[2:] {
				if !filepath.IsAbs(p) && workDir != "" {
					p = filepath.Join(workDir, p)
				}
				paths = append(paths, p)
			}
			for _, p := range a.mergedPeers() {
				if p.DeviceName == name {
					log.Printf("[cli] Sending %d file(s) to %s", len(paths), name)
					a.cliError(a.SendFilePaths(p.IPAddress, p.Port, name, paths))
					return
				}
			}
		}
	}()
}

func (a *App) cliError(err error) {
	if err != nil {
		log.Printf("[cli] %v", err)
		notify(Notification{Title: "PALT", Body: err.Error(), Icon: "dialog-warning"})
	}
}
