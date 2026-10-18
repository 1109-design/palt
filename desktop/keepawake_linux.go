package main

import (
	"sync"

	"github.com/godbus/dbus/v5"
)

var idleInhibit struct {
	sync.Mutex
	conn   *dbus.Conn
	cookie uint32
}

// setIdleInhibit asks GNOME not to go idle (blank, lock, suspend), or
// stops asking.
func setIdleInhibit(on bool) error {
	idleInhibit.Lock()
	defer idleInhibit.Unlock()
	if idleInhibit.conn == nil {
		c, err := dbus.SessionBus()
		if err != nil {
			return err
		}
		idleInhibit.conn = c
	}
	sm := idleInhibit.conn.Object("org.gnome.SessionManager", "/org/gnome/SessionManager")
	if !on {
		if idleInhibit.cookie == 0 {
			return nil
		}
		err := sm.Call("org.gnome.SessionManager.Uninhibit", 0, idleInhibit.cookie).Err
		idleInhibit.cookie = 0
		return err
	}
	if idleInhibit.cookie != 0 {
		return nil
	}
	// Flag 8: inhibit the session going idle.
	return sm.Call("org.gnome.SessionManager.Inhibit", 0,
		"palt", uint32(0), "A paired device is connected", uint32(8)).Store(&idleInhibit.cookie)
}
