package pbx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "profiles.json")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.Put(Profile{Name: " Hospital Juan José Arévalo ", Host: "10.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == "" || a.Name != "Hospital Juan José Arévalo" || a.SSH.Port != 22 || a.AMI.Port != 5038 || a.API.BaseURL != "http://10.0.0.1" {
		t.Fatalf("normalize not applied: %+v", a)
	}
	b, err := s.Put(Profile{Name: "B", Host: "10.0.0.2"})
	if err != nil {
		t.Fatal(err)
	}
	if s.ActiveID() != a.ID {
		t.Fatalf("first profile should become active, got %q", s.ActiveID())
	}
	if err := s.SetActive(b.ID); err != nil {
		t.Fatal(err)
	}

	s2, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s2.Get(a.ID)
	if !ok || got.Name != "Hospital Juan José Arévalo" || len(s2.Profiles()) != 2 || s2.ActiveID() != b.ID {
		t.Fatalf("reload mismatch: %+v", s2.Profiles())
	}
	if err := s2.Delete(b.ID); err != nil {
		t.Fatal(err)
	}
	if s2.ActiveID() != a.ID {
		t.Fatalf("deleting the active profile should fall back to the first, got %q", s2.ActiveID())
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.tmp")); len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

func TestStorePutUnknownID(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "p.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Put(Profile{ID: "nope", Name: "X", Host: "h"}); err == nil {
		t.Fatal("expected an error for an unknown id")
	}
}

func TestStoreRecoversCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Recovered == "" || len(s.Profiles()) != 0 {
		t.Fatalf("want empty store plus backup, got recovered=%q profiles=%d", s.Recovered, len(s.Profiles()))
	}
	raw, err := os.ReadFile(s.Recovered)
	if err != nil || string(raw) != "{not json" {
		t.Fatalf("backup must keep the original bytes: %q %v", raw, err)
	}
}

func TestValidate(t *testing.T) {
	ok := Profile{
		Name: "Hospital Juan José Arévalo", Host: "10.20.1.5",
		API: APIConfig{Enabled: true},
		SSH: SSHConfig{Enabled: true, User: "root"},
		AMI: AMIConfig{Enabled: true, User: "admin"},
	}
	ok.Normalize()
	if err := Validate(ok); err != nil {
		t.Fatalf("valid profile rejected: %v", err)
	}
	bad := map[string]func(p *Profile){
		"scheme in host": func(p *Profile) { p.Host = "https://10.20.1.5" },
		"port in host":   func(p *Profile) { p.Host = "10.20.1.5:8443" },
		"empty name":     func(p *Profile) { p.Name = "   " },
		"ssh port":       func(p *Profile) { p.SSH.Port = 70000 },
		"ami user":       func(p *Profile) { p.AMI.User = "" },
		"api url":        func(p *Profile) { p.API.BaseURL = "ftp://x" },
	}
	for name, mutate := range bad {
		p := ok
		mutate(&p)
		if err := Validate(p); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}
	v6 := ok
	v6.Host, v6.API.BaseURL = "fe80::1", ""
	v6.Normalize()
	if err := Validate(v6); err != nil {
		t.Errorf("bare IPv6 should pass: %v", err)
	}
	off := ok
	off.SSH = SSHConfig{Enabled: false}
	if err := Validate(off); err != nil {
		t.Errorf("a disabled channel needs no user: %v", err)
	}
}

func TestSecretsKeepOnEmpty(t *testing.T) {
	keyring.MockInit()
	if err := StoreSecrets("p1", Secrets{API: "a", SSH: "s", AMI: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := StoreSecrets("p1", Secrets{SSH: "s2"}); err != nil { // edit: only SSH typed
		t.Fatal(err)
	}
	got, err := LoadSecrets("p1")
	if err != nil {
		t.Fatal(err)
	}
	if got != (Secrets{API: "a", SSH: "s2", AMI: "m"}) {
		t.Fatalf("empty fields must keep stored secrets, got %+v", got)
	}
	if err := DeleteSecrets("p1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := LoadSecrets("p1"); got != (Secrets{}) {
		t.Fatalf("delete left secrets behind: %+v", got)
	}
}
