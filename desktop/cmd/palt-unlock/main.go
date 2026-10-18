// palt-unlock lets a paired phone's fingerprint unlock this computer.
//
// It is run by PAM (pam_exec) as root for the lock screen, admin prompts
// (polkit) and sudo. It never trusts the PALT app running in the user's
// session: it makes up a one-time challenge itself, hands it to PALT over a
// socket, PALT shows it on the phone, and the phone signs it with a key
// that only works after a fingerprint. palt-unlock checks the signature with
// the phone's public key, which only root can write (/etc/palt/unlock).
// Anything else (no key, PALT not running, phone away, cancelled, timeout)
// fails, and PAM goes on to the password.
//
//	palt-unlock                   PAM auth (reads PAM_USER, PAM_SERVICE)
//	palt-unlock install <keyfile> as root through pkexec: trust the phone
//	                              key for the calling user, add the PAM lines
//	palt-unlock uninstall         as root through pkexec: forget the key,
//	                              remove the PAM lines when no one uses them
//	palt-unlock status            print "on" or "off" for the calling user
package main

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	keyDir = "/etc/palt/unlock"
	// msgPrefix starts every challenge; the phone signs nothing else with
	// this key. Must match mobile/.../Unlock.kt and desktop/unlock_app.go.
	msgPrefix = "PALT-UNLOCK-1"
	// answerTimeout is how long the phone has to answer before PAM moves on
	// to the password.
	answerTimeout = 25 * time.Second
)

func main() {
	var err error
	switch {
	case len(os.Args) == 1:
		if authenticate() {
			os.Exit(0)
		}
		os.Exit(1)
	case os.Args[1] == "install" && len(os.Args) == 3:
		err = install(os.Args[2])
	case os.Args[1] == "uninstall" && len(os.Args) == 2:
		err = uninstall()
	case os.Args[1] == "status" && len(os.Args) == 2:
		u, uerr := user.Current()
		if uerr == nil && keyFor(u.Username) != nil {
			fmt.Println("on")
		} else {
			fmt.Println("off")
		}
	default:
		err = errors.New("usage: palt-unlock [install <keyfile> | uninstall | status]")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "palt-unlock:", err)
		os.Exit(1)
	}
}

// ─── PAM auth ────────────────────────────────────────────────────────────────

func authenticate() bool {
	if os.Getenv("PAM_TYPE") != "auth" || os.Geteuid() != 0 {
		return false
	}
	name := os.Getenv("PAM_USER")
	pub := keyFor(name)
	if pub == nil {
		return false
	}
	u, err := user.Lookup(name)
	if err != nil {
		return false
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return false
	}
	conn, err := dialPALT(uid)
	if err != nil {
		return false
	}
	defer conn.Close()

	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return false
	}
	host, _ := os.Hostname()
	msg := strings.Join([]string{msgPrefix, host, name, os.Getenv("PAM_SERVICE"), hex.EncodeToString(nonce)}, "\n")

	conn.SetDeadline(time.Now().Add(answerTimeout))
	req, _ := json.Marshal(map[string]string{"message": msg})
	if _, err := conn.Write(append(req, '\n')); err != nil {
		return false
	}
	r := bufio.NewReader(conn)
	var first struct {
		Asking bool   `json:"asking"`
		Sig    string `json:"signature"`
	}
	line, err := r.ReadBytes('\n')
	if err != nil || json.Unmarshal(line, &first) != nil {
		return false
	}
	// PALT says it reached the phone; pam_exec's "stdout" shows this.
	if first.Asking {
		fmt.Println("Confirm with your fingerprint on your phone…")
		line, err = r.ReadBytes('\n')
		if err != nil || json.Unmarshal(line, &first) != nil {
			return false
		}
	}
	sig, err := base64.StdEncoding.DecodeString(first.Sig)
	if err != nil || len(sig) == 0 {
		return false
	}
	digest := sha256.Sum256([]byte(msg))
	return ecdsa.VerifyASN1(pub, digest[:], sig)
}

// dialPALT connects to the PALT app of user uid, making sure the socket
// really belongs to that user's process.
func dialPALT(uid int) (*net.UnixConn, error) {
	path := fmt.Sprintf("/run/user/%d/palt/unlock.sock", uid)
	var st syscall.Stat_t
	if err := syscall.Lstat(path, &st); err != nil {
		return nil, err
	}
	if st.Mode&syscall.S_IFMT != syscall.S_IFSOCK || int(st.Uid) != uid {
		return nil, errors.New("socket not owned by the user")
	}
	c, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		return nil, err
	}
	uc := c.(*net.UnixConn)
	raw, err := uc.SyscallConn()
	if err != nil {
		uc.Close()
		return nil, err
	}
	var cred *syscall.Ucred
	var credErr error
	raw.Control(func(fd uintptr) {
		cred, credErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if credErr != nil || cred == nil || int(cred.Uid) != uid {
		uc.Close()
		return nil, errors.New("socket served by someone else")
	}
	return uc, nil
}

// keyFor returns the trusted phone key of a user, or nil. The file and its
// folders must be root's and not writable by anyone else.
func keyFor(name string) *ecdsa.PublicKey {
	if name == "" || strings.ContainsAny(name, "/\x00") || strings.HasPrefix(name, ".") {
		return nil
	}
	path := filepath.Join(keyDir, name+".pub")
	for _, p := range []string{path, keyDir, filepath.Dir(keyDir)} {
		var st syscall.Stat_t
		if syscall.Lstat(p, &st) != nil || st.Uid != 0 || st.Mode&0o022 != 0 {
			return nil
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	pub, err := parseKey(strings.TrimSpace(string(b)))
	if err != nil {
		return nil
	}
	return pub
}

// parseKey reads a base64 X.509 SubjectPublicKeyInfo holding a P-256 key
// (what Android's Keystore gives).
func parseKey(b64 string) (*ecdsa.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	k, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, err
	}
	pub, ok := k.(*ecdsa.PublicKey)
	if !ok || pub.Curve != elliptic.P256() {
		return nil, errors.New("not a P-256 key")
	}
	return pub, nil
}

// ─── install / uninstall ─────────────────────────────────────────────────────

// callingUser is who ran pkexec; install and uninstall only touch them.
func callingUser() (string, error) {
	if os.Geteuid() != 0 {
		return "", errors.New("run through pkexec")
	}
	id := os.Getenv("PKEXEC_UID")
	if id == "" {
		id = os.Getenv("SUDO_UID")
	}
	if id == "" {
		return "", errors.New("can't tell who is asking (no PKEXEC_UID)")
	}
	u, err := user.LookupId(id)
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

func install(keyFile string) error {
	name, err := callingUser()
	if err != nil {
		return err
	}
	self, err := trustedSelf()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(keyFile)
	if err != nil {
		return err
	}
	key := strings.TrimSpace(string(b))
	if _, err := parseKey(key); err != nil {
		return fmt.Errorf("bad phone key: %w", err)
	}
	if err := os.MkdirAll(keyDir, 0o755); err != nil {
		return err
	}
	os.Chmod(filepath.Dir(keyDir), 0o755)
	os.Chmod(keyDir, 0o755)
	tmp := filepath.Join(keyDir, "."+name+".tmp")
	if err := os.WriteFile(tmp, []byte(key+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, filepath.Join(keyDir, name+".pub")); err != nil {
		return err
	}
	return addPAM(self)
}

func uninstall() error {
	name, err := callingUser()
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(keyDir, name+".pub")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	keys, _ := filepath.Glob(filepath.Join(keyDir, "*.pub"))
	if len(keys) == 0 {
		return removePAM()
	}
	return nil
}

// PAM services PALT unlocks: the lock screen, admin prompts and sudo.
var services = []string{"gdm-password", "polkit-1", "sudo"}

const (
	pamMarker = "# PALT: unlock with your phone's fingerprint (palt-unlock uninstall removes this)"
	createdBy = "# Created by PALT from the system default; palt-unlock uninstall removes it."
)

// pamLine is "sufficient": a good fingerprint skips the password, anything
// else falls through to it.
func pamLine(self string) string {
	return "auth\tsufficient\tpam_exec.so quiet stdout seteuid " + self
}

// trustedSelf is this program's path, if only root can change it: PAM runs
// it as root, so a copy in a user's folder must never be wired in.
func trustedSelf() (string, error) {
	self, err := os.Executable()
	if err == nil {
		self, err = filepath.EvalSymlinks(self)
	}
	if err != nil {
		return "", err
	}
	for p := self; ; p = filepath.Dir(p) {
		var st syscall.Stat_t
		if err := syscall.Stat(p, &st); err != nil {
			return "", err
		}
		if st.Uid != 0 || st.Mode&0o022 != 0 {
			return "", fmt.Errorf("%s must be root's and not writable by others; use the installed PALT", p)
		}
		if p == "/" {
			return self, nil
		}
	}
}

// Where PAM reads its services: /etc overrides the system defaults.
var pamDir, vendorPamDir = "/etc/pam.d", "/usr/lib/pam.d"

func addPAM(self string) error {
	for _, svc := range services {
		path := filepath.Join(pamDir, svc)
		data, err := os.ReadFile(path)
		created := false
		if errors.Is(err, os.ErrNotExist) {
			// Newer systems keep defaults in /usr/lib/pam.d; a copy in
			// /etc/pam.d overrides it.
			data, err = os.ReadFile(filepath.Join(vendorPamDir, svc))
			created = true
		}
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		if strings.Contains(string(data), pamMarker) {
			continue
		}
		at := -1
		for i, l := range lines {
			if strings.HasPrefix(strings.TrimSpace(l), "@include common-auth") {
				at = i
				break
			}
		}
		if at < 0 {
			continue
		}
		out := append([]string{}, lines[:at]...)
		out = append(out, pamMarker, pamLine(self))
		out = append(out, lines[at:]...)
		if created {
			out = append([]string{createdBy}, out...)
		} else if err := os.WriteFile(path+".palt-backup", data, 0o644); err != nil {
			return err
		}
		if err := writeAtomic(path, strings.Join(out, "\n")); err != nil {
			return err
		}
	}
	return nil
}

func removePAM() error {
	for _, svc := range services {
		path := filepath.Join(pamDir, svc)
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), pamMarker) {
			continue
		}
		if strings.HasPrefix(string(data), createdBy) {
			if err := os.Remove(path); err != nil {
				return err
			}
			continue
		}
		var out []string
		skip := false
		for _, l := range strings.Split(string(data), "\n") {
			if l == pamMarker {
				skip = true
				continue
			}
			if skip && strings.Contains(l, "pam_exec.so") && strings.Contains(l, "palt-unlock") {
				skip = false
				continue
			}
			skip = false
			out = append(out, l)
		}
		if err := writeAtomic(path, strings.Join(out, "\n")); err != nil {
			return err
		}
		os.Remove(path + ".palt-backup")
	}
	return nil
}

func writeAtomic(path, content string) error {
	tmp := path + ".palt-tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
