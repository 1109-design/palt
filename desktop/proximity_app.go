// proximity_app.go locks this computer when a phone with "proximityLock" on
// goes away: its link is gone, or only reaches us through the relay, for
// proximityGrace. No location is used. It never unlocks; the phone's
// fingerprint does that (unlock_app.go).
package main

import (
	"log"
	"time"

	"palt/internal/link"
)

const proximityGrace = 2 * time.Minute

func (a *App) setupProximityLock() {
	go func() {
		seenNear := map[string]bool{}       // near at least once, so a cold start doesn't lock
		awaySince := map[string]time.Time{} // when it left
		done := map[string]bool{}           // already locked for this absence
		for range time.Tick(10 * time.Second) {
			near := map[string]bool{}
			for _, l := range a.links.Links() {
				if l.Route != link.RouteRelay {
					near[l.Peer.Name] = true
				}
			}
			settingsMu.Lock()
			all := loadDeviceSettings()
			settingsMu.Unlock()
			for name, s := range all {
				if !s["proximityLock"] {
					continue
				}
				if near[name] {
					seenNear[name] = true
					delete(awaySince, name)
					done[name] = false
					continue
				}
				if !seenNear[name] || done[name] {
					continue
				}
				since, ok := awaySince[name]
				if !ok {
					awaySince[name] = time.Now()
					continue
				}
				if time.Since(since) < proximityGrace {
					continue
				}
				done[name] = true
				if screenLocked() {
					continue
				}
				log.Printf("[proximity] %s left, locking", name)
				if err := lockScreen(); err != nil {
					log.Printf("[proximity] lock: %v", err)
				}
			}
		}
	}()
}
