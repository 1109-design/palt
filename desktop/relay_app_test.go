package main

import (
	"testing"
	"time"

	"palt/internal/relay"
)

// Switching or clearing the pairing code while a client is running must not
// deadlock (Stop fires callbacks that read the current client).
func TestRelayRestartDoesNotDeadlock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := NewApp()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 3; i++ {
			code, _ := relay.NewCode("127.0.0.1:1")
			if err := a.SetRelayCode(code.String()); err != nil {
				t.Error(err)
			}
		}
		if st := a.GetRelayStatus(); !st.Configured {
			t.Errorf("status not configured: %+v", st)
		}
		a.SetRelayCode("")
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
