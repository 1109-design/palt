// unlock_app.go lets a paired phone's fingerprint unlock this computer: the
// lock screen, admin prompts and sudo.
//
// The checking is done by palt-unlock (cmd/palt-unlock), which PAM runs as
// root. It sends a one-time challenge to this app over a socket in the
// user's runtime folder; the app passes it to the phone, which signs it
// after a fingerprint; palt-unlock checks the signature with the phone's
// public key saved under /etc/palt at setup. The app only carries messages,
// so nothing in the user's session can unlock by itself.
//
// Packet types (must match mobile/lib/features/unlock.dart). Requests carry
// an id and are answered by "<type>.reply" with the same id.
//
//	unlock.setup   desktop → phone  {} → {publicKey} (base64 X.509, P-256)
//	unlock.sign    desktop → phone  {message} → {signature} (base64 ASN.1)
//	unlock.forget  desktop → phone  {} → {}
package main

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// unlockPrefix starts every challenge (cmd/palt-unlock/main.go).
	unlockPrefix = "PALT-UNLOCK-1"
	// unlockAsk is how long the phone has for a fingerprint.
	unlockAsk = 22 * time.Second
)

// unlockHelpers are where the installed palt-unlock may live.
var unlockHelpers = []string{"/usr/local/bin/palt-unlock", "/usr/bin/palt-unlock"}

// UnlockStatus is the "Unlock with phone" setting.
type UnlockStatus struct {
	Supported bool   `json:"supported"` // Linux with palt-unlock installed
	Enabled   bool   `json:"enabled"`   // a phone key is trusted for this user
	Device    string `json:"device"`    // the phone's name
	DeviceID  string `json:"deviceId"`
}

type unlockConfig struct {
	DeviceID string `json:"deviceId"`
	Device   string `json:"device"`
}

func unlockHelper() string {
	for _, p := range unlockHelpers {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func loadUnlockConfig() unlockConfig {
	var c unlockConfig
	if dir, err := configDir(); err == nil {
		if b, err := os.ReadFile(filepath.Join(dir, "unlock.json")); err == nil {
			json.Unmarshal(b, &c)
		}
	}
	return c
}

func saveUnlockConfig(c *unlockConfig) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "unlock.json")
	if c == nil {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	b, _ := json.Marshal(c)
	return os.WriteFile(path, b, 0o600)
}

// setupUnlock listens for palt-unlock's challenges.
func (a *App) setupUnlock() {
	if runtime.GOOS != "linux" {
		return
	}
	dir := fmt.Sprintf("/run/user/%d/palt", os.Getuid())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Printf("[unlock] %v", err)
		return
	}
	path := filepath.Join(dir, "unlock.sock")
	os.Remove(path) // left over from a PALT that didn't shut down
	ln, err := net.Listen("unix", path)
	if err != nil {
		log.Printf("[unlock] %v", err)
		return
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go a.serveUnlock(c)
		}
	}()
}

// serveUnlock passes one challenge to the phone and its answer back.
func (a *App) serveUnlock(c net.Conn) {
	defer c.Close()
	c.SetDeadline(time.Now().Add(unlockAsk + 5*time.Second))
	line, err := bufio.NewReader(c).ReadBytes('\n')
	if err != nil {
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	reply := func(v any) {
		b, _ := json.Marshal(v)
		c.Write(append(b, '\n'))
	}
	if json.Unmarshal(line, &req) != nil || !strings.HasPrefix(req.Message, unlockPrefix+"\n") {
		reply(map[string]string{"error": "bad request"})
		return
	}
	cfg := loadUnlockConfig()
	l := a.links.Get(cfg.DeviceID)
	if cfg.DeviceID == "" || l == nil || !l.Supports("unlock.sign") {
		reply(map[string]string{"error": "phone not connected"})
		return
	}
	reply(map[string]bool{"asking": true})
	sig, err := a.unlockSign(cfg.DeviceID, req.Message)
	if err != nil {
		log.Printf("[unlock] %s: %v", cfg.Device, err)
		reply(map[string]string{"error": err.Error()})
		return
	}
	log.Printf("[unlock] %s answered", cfg.Device)
	reply(map[string]string{"signature": sig})
}

func (a *App) unlockSign(deviceID, message string) (string, error) {
	l := a.links.Get(deviceID)
	if l == nil {
		return "", errors.New("phone not connected")
	}
	p, err := l.Request("unlock.sign", map[string]string{"message": message}, unlockAsk)
	if err != nil {
		return "", err
	}
	var r struct {
		Signature string `json:"signature"`
	}
	if err := smsReply(p, &r); err != nil {
		return "", err
	}
	return r.Signature, nil
}

// ─── Wails bindings ──────────────────────────────────────────────────────────

// GetUnlockStatus says whether a phone unlocks this computer.
func (a *App) GetUnlockStatus() UnlockStatus {
	helper := unlockHelper()
	if runtime.GOOS != "linux" || helper == "" {
		return UnlockStatus{}
	}
	s := UnlockStatus{Supported: true}
	out, err := exec.Command(helper, "status").Output()
	if err == nil && strings.TrimSpace(string(out)) == "on" {
		cfg := loadUnlockConfig()
		s.Enabled, s.Device, s.DeviceID = true, cfg.Device, cfg.DeviceID
	}
	return s
}

// SetupUnlock makes the phone's fingerprint unlock this computer. The phone
// makes a key and proves it works with one fingerprint; then the system asks
// for the password once to trust it.
func (a *App) SetupUnlock(device string) error {
	helper := unlockHelper()
	if helper == "" {
		return errors.New("install PALT from its .deb to unlock with the phone")
	}
	l, err := a.linkByName(device)
	if err != nil {
		return err
	}
	if !l.Supports("unlock.setup") {
		return errors.New("update PALT on " + device + " to unlock with it")
	}
	p, err := l.Request("unlock.setup", map[string]any{}, smsRequestTimeout)
	if err != nil {
		return err
	}
	var r struct {
		PublicKey string `json:"publicKey"`
	}
	if err := smsReply(p, &r); err != nil {
		return err
	}
	der, err := base64.StdEncoding.DecodeString(r.PublicKey)
	if err != nil {
		return errors.New("the phone sent a bad key")
	}
	k, err := x509.ParsePKIXPublicKey(der)
	pub, ok := k.(*ecdsa.PublicKey)
	if err != nil || !ok {
		return errors.New("the phone sent a bad key")
	}

	// One fingerprint now shows it works before the system trusts the key.
	nonce := make([]byte, 32)
	rand.Read(nonce)
	host, _ := os.Hostname()
	me, _ := user.Current()
	msg := strings.Join([]string{unlockPrefix, host, me.Username, "setup", hex.EncodeToString(nonce)}, "\n")
	sigB64, err := a.unlockSign(l.Peer.ID, msg)
	if err != nil {
		return err
	}
	sig, _ := base64.StdEncoding.DecodeString(sigB64)
	digest := sha256.Sum256([]byte(msg))
	if !ecdsa.VerifyASN1(pub, digest[:], sig) {
		return errors.New("the phone's answer didn't check out")
	}

	dir, err := configDir()
	if err != nil {
		return err
	}
	keyFile := filepath.Join(dir, "unlock-key.pub")
	if err := os.WriteFile(keyFile, []byte(r.PublicKey+"\n"), 0o644); err != nil {
		return err
	}
	defer os.Remove(keyFile)
	if out, err := exec.Command("pkexec", helper, "install", keyFile).CombinedOutput(); err != nil {
		return pkexecError(out, err)
	}
	return saveUnlockConfig(&unlockConfig{DeviceID: l.Peer.ID, Device: l.Peer.Name})
}

// DisableUnlock stops the phone unlocking this computer.
func (a *App) DisableUnlock() error {
	helper := unlockHelper()
	if helper == "" {
		return errors.New("palt-unlock isn't installed")
	}
	if out, err := exec.Command("pkexec", helper, "uninstall").CombinedOutput(); err != nil {
		return pkexecError(out, err)
	}
	cfg := loadUnlockConfig()
	if l := a.links.Get(cfg.DeviceID); l != nil && l.Supports("unlock.forget") {
		go l.Request("unlock.forget", map[string]any{}, 10*time.Second)
	}
	return saveUnlockConfig(nil)
}

func pkexecError(out []byte, err error) error {
	var ee *exec.ExitError
	// 126: the password dialog was dismissed.
	if errors.As(err, &ee) && (ee.ExitCode() == 126 || ee.ExitCode() == 127) {
		return errors.New("cancelled")
	}
	if msg := strings.TrimSpace(string(out)); msg != "" {
		return errors.New(strings.TrimPrefix(msg, "palt-unlock: "))
	}
	return err
}
