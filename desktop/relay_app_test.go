package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"palt/internal/relay"
)

// Switching or clearing the relay while a client is running must not
// deadlock (Stop fires callbacks that read the current client).
func TestRelayRestartDoesNotDeadlock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := NewApp()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 3; i++ {
			if err := a.SetRelayServer("127.0.0.1:1"); err != nil {
				t.Error(err)
			}
			if err := a.ResetPairing(); err != nil {
				t.Error(err)
			}
		}
		if st := a.GetRelayStatus(); !st.Configured {
			t.Errorf("status not configured: %+v", st)
		}
		a.SetRelayServer("")
		if st := a.GetRelayStatus(); st.Configured || st.Status != relay.StatusOff {
			t.Errorf("status after turning off: %+v", st)
		}
		a.stopRelay()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock switching pairing codes")
	}
}

// A 1.x relay config keeps its device ID and group.
func TestPairingMigratesRelayConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	code, _ := relay.NewCode("relay.example.com:9000")
	dir := filepath.Join(home, ".config", "palt")
	os.MkdirAll(dir, 0o755)
	old, _ := json.Marshal(map[string]string{"code": code.String(), "deviceId": "0123456789abcdef"})
	os.WriteFile(filepath.Join(dir, "relay.json"), old, 0o600)

	p := NewApp().pairing()
	if p.DeviceID != "0123456789abcdef" || !bytes.Equal(p.Key, code.Key) || p.Relay != "relay.example.com:9000" {
		t.Fatalf("migrated pairing: %+v", p)
	}
	// Saved, so a second start gives the same identity.
	if again := NewApp().pairing(); again.DeviceID != p.DeviceID || !bytes.Equal(again.Key, p.Key) {
		t.Fatal("pairing not persisted")
	}

	// Joining another device's group keeps the device ID.
	other := NewApp()
	other.pair = p
	other.pair.Key = make([]byte, 32)
	a := NewApp()
	if err := a.JoinPairing(other.GetPairing().Code); err != nil {
		t.Fatal(err)
	}
	if got := a.pairing(); got.DeviceID != p.DeviceID || !bytes.Equal(got.Key, other.pair.Key) {
		t.Fatalf("after join: %+v", got)
	}
}
