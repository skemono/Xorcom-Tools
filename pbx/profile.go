// Package pbx holds PBX profiles and the clients that talk to CompletePBX (API, SSH, AMI).
package pbx

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

type APIConfig struct {
	Enabled    bool   `json:"enabled"`
	BaseURL    string `json:"baseURL"`
	User       string `json:"user"`
	CertSHA256 string `json:"certSHA256"`
}

type SSHConfig struct {
	Enabled       bool   `json:"enabled"`
	Port          int    `json:"port"`
	User          string `json:"user"`
	KeyPath       string `json:"keyPath"`
	HostKeySHA256 string `json:"hostKeySHA256"`
}

type AMIConfig struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	User    string `json:"user"`
}

type Profile struct {
	ID   string    `json:"id"`
	Name string    `json:"name"`
	Host string    `json:"host"`
	API  APIConfig `json:"api"`
	SSH  SSHConfig `json:"ssh"`
	AMI  AMIConfig `json:"ami"`
}

// Normalize trims text fields and fills default ports and the API base URL.
// CompletePBX 5 serves its portal API on plain HTTP port 80, hence the http:// default.
func (p *Profile) Normalize() {
	p.Name, p.Host = strings.TrimSpace(p.Name), strings.TrimSpace(p.Host)
	p.API.BaseURL = strings.TrimRight(strings.TrimSpace(p.API.BaseURL), "/")
	if p.SSH.Port == 0 {
		p.SSH.Port = 22
	}
	if p.AMI.Port == 0 {
		p.AMI.Port = 5038
	}
	if p.API.BaseURL == "" && p.Host != "" {
		host := p.Host
		if strings.Contains(host, ":") { // bare IPv6
			host = "[" + host + "]"
		}
		p.API.BaseURL = "http://" + host
	}
}

// Validate rejects profiles the channels cannot use. Messages are shown to the user.
func Validate(p Profile) error {
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("el nombre es obligatorio")
	}
	host := strings.TrimSpace(p.Host)
	if host == "" {
		return errors.New("el host es obligatorio")
	}
	if _, _, err := net.SplitHostPort(host); err == nil || strings.ContainsAny(host, "/\\ ") {
		return errors.New("escriba solo el nombre o la IP del host, sin http:// ni puerto (el puerto va en cada canal)")
	}
	if p.API.Enabled {
		u, err := url.Parse(p.API.BaseURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return errors.New("la URL base de la API debe empezar con http:// o https://")
		}
	}
	for _, c := range []struct {
		on   bool
		name string
		port int
		user string
	}{
		{p.SSH.Enabled, "SSH", p.SSH.Port, p.SSH.User},
		{p.AMI.Enabled, "AMI", p.AMI.Port, p.AMI.User},
	} {
		if !c.on {
			continue
		}
		if c.port < 1 || c.port > 65535 {
			return fmt.Errorf("puerto %s inválido: %d", c.name, c.port)
		}
		if strings.TrimSpace(c.user) == "" {
			return fmt.Errorf("falta el usuario %s", c.name)
		}
	}
	return nil
}

type fileData struct {
	Active   string    `json:"active"`
	Folio    int       `json:"folio"` // last work-order number issued
	Profiles []Profile `json:"profiles"`
}

// Store persists profiles as JSON. Not safe for concurrent use; callers serialize access.
type Store struct {
	path string
	data fileData
	// Recovered is the backup path when an unreadable file was set aside on open.
	Recovered string
}

// DefaultPath is %APPDATA%\UtilidadesXorcom\profiles.json on Windows.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "UtilidadesXorcom", "profiles.json"), nil
}

// OpenStore loads path; a missing file is an empty store, an unreadable one is renamed aside, never overwritten.
func OpenStore(path string) (*Store, error) {
	s := &Store{path: path}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		backup := fmt.Sprintf("%s.bad-%s", path, time.Now().Format("20060102-150405"))
		if err := os.Rename(path, backup); err != nil {
			return nil, err
		}
		s.data, s.Recovered = fileData{}, backup
	}
	return s, nil
}

func (s *Store) Profiles() []Profile { return append([]Profile(nil), s.data.Profiles...) }
func (s *Store) ActiveID() string     { return s.data.Active }

func (s *Store) Get(id string) (Profile, bool) {
	if i := s.index(id); i >= 0 {
		return s.data.Profiles[i], true
	}
	return Profile{}, false
}

// Put inserts (empty ID) or replaces a profile and saves. The first profile becomes active.
func (s *Store) Put(p Profile) (Profile, error) {
	p.Normalize()
	if p.ID == "" {
		p.ID = newID()
		s.data.Profiles = append(s.data.Profiles, p)
	} else {
		i := s.index(p.ID)
		if i < 0 {
			return Profile{}, fmt.Errorf("el perfil %q no existe", p.ID)
		}
		s.data.Profiles[i] = p
	}
	if s.data.Active == "" {
		s.data.Active = p.ID
	}
	return p, s.save()
}

func (s *Store) Delete(id string) error {
	i := s.index(id)
	if i < 0 {
		return fmt.Errorf("el perfil %q no existe", id)
	}
	s.data.Profiles = append(s.data.Profiles[:i], s.data.Profiles[i+1:]...)
	if s.data.Active == id {
		s.data.Active = ""
		if len(s.data.Profiles) > 0 {
			s.data.Active = s.data.Profiles[0].ID
		}
	}
	return s.save()
}

// NextFolio issues the next work-order number; it keeps counting across sessions so no folio repeats.
func (s *Store) NextFolio() (int, error) {
	s.data.Folio++
	return s.data.Folio, s.save()
}

func (s *Store) SetActive(id string) error {
	if s.index(id) < 0 {
		return fmt.Errorf("el perfil %q no existe", id)
	}
	s.data.Active = id
	return s.save()
}

func (s *Store) index(id string) int {
	for i, p := range s.data.Profiles {
		if p.ID == id {
			return i
		}
	}
	return -1
}

// save writes a temp file next to the target and renames it over, so a crash never leaves half a file.
func (s *Store) save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "profiles-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

func newID() string {
	b := make([]byte, 6)
	rand.Read(b) // crypto/rand.Read does not fail on supported platforms
	return hex.EncodeToString(b)
}

// Secrets travel UI -> Go only. An empty field means "keep what is stored".
type Secrets struct {
	API    string `json:"api"`
	SSH    string `json:"ssh"`
	SSHKey string `json:"sshKey"` // passphrase of the SSH key file
	AMI    string `json:"ami"`
}

const keyringService = "UtilidadesXorcom"

var secretKinds = []string{"api", "ssh", "sshkey", "ami"}

func (s *Secrets) fields() []*string { return []*string{&s.API, &s.SSH, &s.SSHKey, &s.AMI} }

func LoadSecrets(profileID string) (Secrets, error) {
	var s Secrets
	for i, f := range s.fields() {
		v, err := keyring.Get(keyringService, profileID+"/"+secretKinds[i])
		if err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return s, err
		}
		*f = v
	}
	return s, nil
}

func StoreSecrets(profileID string, s Secrets) error {
	for i, f := range s.fields() {
		if *f == "" {
			continue
		}
		if err := keyring.Set(keyringService, profileID+"/"+secretKinds[i], *f); err != nil {
			return err
		}
	}
	return nil
}

func DeleteSecrets(profileID string) error {
	for _, kind := range secretKinds {
		if err := keyring.Delete(keyringService, profileID+"/"+kind); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return err
		}
	}
	return nil
}
