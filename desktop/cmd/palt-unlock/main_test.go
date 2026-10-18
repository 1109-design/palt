package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const gdmPassword = `auth    requisite       pam_nologin.so
auth	required	pam_succeed_if.so user != root quiet_success
@include common-auth
auth    optional        pam_gnome_keyring.so
@include common-account
`

const polkit = `#%PAM-1.0

@include common-auth
@include common-account
`

func TestPAMRoundTrip(t *testing.T) {
	etc, vendor := t.TempDir(), t.TempDir()
	pamDir, vendorPamDir = etc, vendor
	os.WriteFile(filepath.Join(etc, "gdm-password"), []byte(gdmPassword), 0o644)
	os.WriteFile(filepath.Join(vendor, "polkit-1"), []byte(polkit), 0o644)

	self := "/usr/local/bin/palt-unlock"
	for range 2 { // a second install changes nothing
		if err := addPAM(self); err != nil {
			t.Fatal(err)
		}
	}
	gdm, _ := os.ReadFile(filepath.Join(etc, "gdm-password"))
	lines := strings.Split(string(gdm), "\n")
	if lines[2] != pamMarker || lines[3] != pamLine(self) || lines[4] != "@include common-auth" {
		t.Fatalf("gdm-password:\n%s", gdm)
	}
	if strings.Count(string(gdm), pamMarker) != 1 {
		t.Fatal("added twice")
	}
	pk, err := os.ReadFile(filepath.Join(etc, "polkit-1"))
	if err != nil || !strings.HasPrefix(string(pk), createdBy) || !strings.Contains(string(pk), pamLine(self)) {
		t.Fatalf("polkit-1 not overridden:\n%s", pk)
	}
	if _, err := os.Stat(filepath.Join(etc, "sudo")); err == nil {
		t.Fatal("made up a sudo file")
	}

	if err := removePAM(); err != nil {
		t.Fatal(err)
	}
	gdm, _ = os.ReadFile(filepath.Join(etc, "gdm-password"))
	if string(gdm) != gdmPassword {
		t.Fatalf("gdm-password not restored:\n%s", gdm)
	}
	if _, err := os.Stat(filepath.Join(etc, "polkit-1")); err == nil {
		t.Fatal("polkit-1 override left behind")
	}
	if _, err := os.Stat(filepath.Join(etc, "gdm-password.palt-backup")); err == nil {
		t.Fatal("backup left behind")
	}
}

func TestParseKeyRejectsJunk(t *testing.T) {
	for _, k := range []string{"", "bm90IGEga2V5", "MCowBQYDK2VwAyEAGb9ECWmEzf6FQbrBZ9w7lshQhqowtrbLDFw4rXAxZuE="} {
		if _, err := parseKey(k); err == nil {
			t.Errorf("accepted %q", k)
		}
	}
}
