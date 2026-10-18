// keepawake_app.go keeps the computer from blanking or suspending while a
// device with "keepAwake" on is linked (off by default).
package main

import "time"

func (a *App) setupKeepAwake() {
	go func() {
		held := false
		for range time.Tick(5 * time.Second) {
			want := false
			for _, l := range a.links.Links() {
				if a.setting(l.Peer.Name, "keepAwake") {
					want = true
					break
				}
			}
			if want != held && setIdleInhibit(want) == nil {
				held = want
			}
		}
	}()
}
