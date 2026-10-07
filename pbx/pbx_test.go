package pbx

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/ssh"
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
	if a.ID == "" || a.Name != "Hospital Juan José Arévalo" || a.SSH.Port != 22 || a.AMI.Port != 5038 || a.APIBase() != "http://10.0.0.1" {
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

// Final review Important 1: a host correction must move the API target too, or the portal
// password keeps going (in cleartext) to whatever answers at the old address.
func TestAPIBaseFollowsHostEdit(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "p.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Put(Profile{Name: "A", Host: "10.20.1.5", API: APIConfig{Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	if p.API.BaseURL != "" {
		t.Fatalf("the derived default must not be persisted, got %q", p.API.BaseURL)
	}
	p.Host = "10.20.1.15"
	if p, err = s.Put(p); err != nil {
		t.Fatal(err)
	}
	if got := p.APIBase(); got != "http://10.20.1.15" {
		t.Fatalf("API target must follow the host edit, got %q", got)
	}
	custom := Profile{Host: "10.20.1.5", API: APIConfig{BaseURL: "https://pbx.local:8443/"}}
	custom.Normalize()
	if got := custom.APIBase(); got != "https://pbx.local:8443" {
		t.Fatalf("a typed base URL wins (trailing slash trimmed), got %q", got)
	}
	if got := (Profile{Host: "fe80::1"}).APIBase(); got != "http://[fe80::1]" {
		t.Fatalf("bare IPv6 must be bracketed, got %q", got)
	}
}

func TestStoreFolioPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	for want := 1; want <= 2; want++ {
		if got, err := s.NextFolio(); err != nil || got != want {
			t.Fatalf("NextFolio = %d, %v; want %d", got, err, want)
		}
	}
	s2, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := s2.NextFolio(); got != 3 {
		t.Fatalf("folio must keep counting across sessions, got %d", got)
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

func TestPinnedTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	fp := CertFingerprint(srv.Certificate().Raw)
	ctx := context.Background()
	p := Profile{Host: "127.0.0.1", API: APIConfig{Enabled: true, BaseURL: srv.URL}}

	var ue *UntrustedError
	if _, err := APICheck(ctx, p, ""); !errors.As(err, &ue) || ue.Fingerprint != fp || ue.Changed {
		t.Fatalf("unpinned self-signed cert: want UntrustedError{%s}, got %v", fp, err)
	}
	p.API.CertSHA256 = fp
	if _, err := APICheck(ctx, p, ""); err != nil {
		t.Fatalf("pinned cert must pass: %v", err)
	}
	p.API.CertSHA256 = strings.Repeat("AB:", 31) + "AB"
	if _, err := APICheck(ctx, p, ""); !errors.As(err, &ue) || !ue.Changed {
		t.Fatalf("different pin: want Changed, got %v", err)
	}
}

func TestAPICheckServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	p := Profile{Host: "127.0.0.1", API: APIConfig{Enabled: true, BaseURL: srv.URL}}
	if _, err := APICheck(context.Background(), p, ""); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("want a 503 error, got %v", err)
	}
}

// fakePortal mimics the CompletePBX 5 portal login: POST /login answers JSON and sets "sid" on success.
func fakePortal(t *testing.T, setSID bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/apply-changes" {
			if c, err := r.Cookie("sid"); err != nil || c.Value != "abc" || r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
				w.Write([]byte("<html>login page</html>"))
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"state": "success", "action": "sysreload-applied",
				"notification": map[string]string{"text": "The system has been reloaded with all outstanding changes"}})
			return
		}
		if r.URL.Path != "/login" || r.Method != http.MethodPost || r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
			w.Write([]byte("<html>login page</html>")) // what the portal does for non-AJAX requests
			return
		}
		r.ParseForm()
		if r.Form.Get("baseurl") != "http://"+r.Host {
			t.Errorf("baseurl = %q, want http://%s", r.Form.Get("baseurl"), r.Host)
		}
		if r.Form.Get("userid") == "admin" && r.Form.Get("userpass") == "good" {
			if setSID {
				http.SetCookie(w, &http.Cookie{Name: "sid", Value: "abc", Path: "/"})
			}
			json.NewEncoder(w).Encode(map[string]any{"state": "success"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"state": "exception", "notification": map[string]string{"text": "Usuario o contraseña incorrecta"}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAPICheckLogin(t *testing.T) {
	ctx := context.Background()
	srv := fakePortal(t, true)
	p := Profile{Host: "127.0.0.1", API: APIConfig{Enabled: true, BaseURL: srv.URL + "/"}} // user empty → admin
	p.Normalize()
	if msg, err := APICheck(ctx, p, "good"); err != nil || !strings.Contains(msg, "admin") {
		t.Fatalf("want login success as admin, got %q %v", msg, err)
	}
	if _, err := APICheck(ctx, p, "bad"); err == nil || !strings.Contains(err.Error(), "Usuario o contraseña incorrecta") {
		t.Fatalf("want the portal's rejection text, got %v", err)
	}
	noSID := fakePortal(t, false)
	p.API.BaseURL = noSID.URL
	if _, err := APICheck(ctx, p, "good"); err == nil {
		t.Fatal("state success without a sid cookie must not count as logged in")
	}
}

func TestPortalApplyChanges(t *testing.T) {
	ctx := context.Background()
	p := Profile{Host: "127.0.0.1", API: APIConfig{Enabled: true, BaseURL: fakePortal(t, true).URL}}
	msg, err := PortalApplyChanges(ctx, p, "good")
	if err != nil || !strings.Contains(msg, "reloaded") {
		t.Fatalf("want the portal's notification, got %q %v", msg, err)
	}
	if _, err := PortalApplyChanges(ctx, p, "bad"); err == nil {
		t.Fatal("without a session there must be no apply")
	}
}

// fakeAMI serves one connection: banner, an unsolicited event, then Success if the secret is "ok".
func fakeAMI(t *testing.T, banner string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		io.WriteString(c, banner+"\r\n")
		r := bufio.NewReader(c)
		secret := ""
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if v, ok := strings.CutPrefix(line, "Secret: "); ok {
				secret = v
			}
			if line == "" {
				break
			}
		}
		io.WriteString(c, "Event: FullyBooted\r\nStatus: Fully Booted\r\n\r\n")
		if secret == "ok" {
			io.WriteString(c, "Response: Success\r\nMessage: Authentication accepted\r\n\r\n")
		} else {
			io.WriteString(c, "Response: Error\r\nMessage: Authentication failed\r\n\r\n")
		}
		io.Copy(io.Discard, r) // wait for Logoff / close
	}()
	return ln.Addr().String()
}

func TestAMILogin(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if banner, err := AMILogin(ctx, fakeAMI(t, "Asterisk Call Manager/7.0.3"), "admin", "ok"); err != nil || banner != "Asterisk Call Manager/7.0.3" {
		t.Fatalf("want success, got %q %v", banner, err)
	}
	if _, err := AMILogin(ctx, fakeAMI(t, "Asterisk Call Manager/7.0.3"), "admin", "bad"); err == nil || !strings.Contains(err.Error(), "Authentication failed") {
		t.Fatalf("want auth failure, got %v", err)
	}
	if _, err := AMILogin(ctx, fakeAMI(t, "SSH-2.0-OpenSSH_9.2"), "admin", "ok"); err == nil {
		t.Fatal("a non-AMI banner must fail")
	}
	// Review focus 2: CR/LF must be rejected before any dial (the address is unroutable on purpose).
	if _, err := AMILogin(ctx, "192.0.2.1:5038", "admin", "x\r\nAction: Originate"); err == nil || !strings.Contains(err.Error(), "saltos de línea") {
		t.Fatalf("want CR/LF rejection, got %v", err)
	}
}

func TestHostKeyCheck(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	fp := ssh.FingerprintSHA256(key)
	var ue *UntrustedError
	if err := HostKeyCheck("")("pbx:22", nil, key); !errors.As(err, &ue) || ue.Fingerprint != fp || ue.Changed {
		t.Fatalf("unpinned: want UntrustedError{%s}, got %v", fp, err)
	}
	if err := HostKeyCheck("SHA256:something-else")("pbx:22", nil, key); !errors.As(err, &ue) || !ue.Changed {
		t.Fatalf("different pin: want Changed, got %v", err)
	}
	if err := HostKeyCheck(fp)("pbx:22", nil, key); err != nil {
		t.Fatalf("pinned key must pass: %v", err)
	}
}

func TestDescribe(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{context.DeadlineExceeded, "sin respuesta en 10 s"},
		{fmt.Errorf("dial tcp: %w", &net.DNSError{Name: "pbx.local", Err: "no such host"}), "no se encontró el host pbx.local"},
		{errors.New("connectex: No connection could be made because the target machine actively refused it."), "conexión rechazada"},
		{&UntrustedError{Fingerprint: "x", Changed: true}, "la huella cambió"},
		{errors.New("ssh: handshake failed: ssh: unable to authenticate"), "usuario o contraseña SSH incorrectos"},
	}
	for _, c := range cases {
		if got := Describe(c.err); !strings.Contains(got, c.want) {
			t.Errorf("Describe(%v) = %q, want it to contain %q", c.err, got, c.want)
		}
	}
}

func TestCheckSkipsDisabledAndIsolatesFailures(t *testing.T) {
	host, port, _ := net.SplitHostPort(fakeAMI(t, "Asterisk Call Manager/7.0.3"))
	amiPort, _ := strconv.Atoi(port)
	p := Profile{
		Host: host,
		API:  APIConfig{Enabled: true, BaseURL: "http://127.0.0.1:1"}, // closed port: fails
		AMI:  AMIConfig{Enabled: true, Port: amiPort, User: "admin"},
	}
	res := Check(context.Background(), p, Secrets{AMI: "ok"})
	if len(res) != 3 || res[0].Channel != "api" || res[1].Channel != "ssh" || res[2].Channel != "ami" {
		t.Fatalf("want api, ssh, ami in order, got %+v", res)
	}
	if res[0].OK || res[0].Message == "" {
		t.Fatalf("api on a closed port must fail with a message: %+v", res[0])
	}
	if !res[1].Skipped || res[1].OK {
		t.Fatalf("disabled ssh must be skipped: %+v", res[1])
	}
	if !res[2].OK || !strings.Contains(res[2].Message, "Asterisk Call Manager") {
		t.Fatalf("ami must succeed despite the api failure: %+v", res[2])
	}
}
