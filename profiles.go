package main

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/skemono/Xorcom-Tools/pbx"
)

// ProfileService owns the profile store. The frontend never receives stored secrets.
type ProfileService struct {
	mu    sync.Mutex
	store *pbx.Store
}

type SecretFlags struct {
	API    bool `json:"api"`
	SSH    bool `json:"ssh"`
	SSHKey bool `json:"sshKey"`
	AMI    bool `json:"ami"`
}

type ProfileView struct {
	Profile pbx.Profile `json:"profile"`
	Has     SecretFlags `json:"has"`
}

type ProfilesView struct {
	Active    string        `json:"active"`
	Profiles  []ProfileView `json:"profiles"`
	Recovered string        `json:"recovered"` // backup path of a corrupt profiles file, reported once
}

func NewProfileService() (*ProfileService, error) {
	path, err := pbx.DefaultPath()
	if err != nil {
		return nil, err
	}
	st, err := pbx.OpenStore(path)
	if err != nil {
		return nil, err
	}
	return &ProfileService{store: st}, nil
}

func (s *ProfileService) List() ProfilesView {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := ProfilesView{Active: s.store.ActiveID(), Profiles: []ProfileView{}, Recovered: s.store.Recovered}
	s.store.Recovered = ""
	for _, p := range s.store.Profiles() {
		sec, _ := pbx.LoadSecrets(p.ID) // a keyring read failure just shows the secret as missing
		v.Profiles = append(v.Profiles, ProfileView{Profile: p, Has: SecretFlags{
			API: sec.API != "", SSH: sec.SSH != "", SSHKey: sec.SSHKey != "", AMI: sec.AMI != "",
		}})
	}
	return v
}

// Save creates (empty ID) or updates a profile. Empty secret fields keep the stored secrets.
func (s *ProfileService) Save(p pbx.Profile, sec pbx.Secrets) (pbx.Profile, error) {
	p.Normalize()
	if err := pbx.Validate(p); err != nil {
		return pbx.Profile{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	saved, err := s.store.Put(p)
	if err != nil {
		return pbx.Profile{}, err
	}
	if err := pbx.StoreSecrets(saved.ID, sec); err != nil {
		return saved, fmt.Errorf("perfil guardado, pero no se pudo guardar la contraseña: %w", err)
	}
	return saved, nil
}

func (s *ProfileService) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.store.Delete(id); err != nil {
		return err
	}
	return pbx.DeleteSecrets(id)
}

func (s *ProfileService) SetActive(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.SetActive(id)
}

// Test checks the saved profile's enabled channels (not unsaved form edits).
func (s *ProfileService) Test(id string) ([]pbx.ChannelResult, error) {
	s.mu.Lock()
	p, ok := s.store.Get(id)
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("el perfil %q no existe", id)
	}
	sec, err := pbx.LoadSecrets(id)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer las contraseñas: %w", err)
	}
	return pbx.Check(context.Background(), p, sec), nil
}

// TrustFingerprint pins the certificate ("api") or host key ("ssh") the user just accepted.
func (s *ProfileService) TrustFingerprint(id, channel, fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.store.Get(id)
	if !ok {
		return fmt.Errorf("el perfil %q no existe", id)
	}
	switch channel {
	case "api":
		p.API.CertSHA256 = fingerprint
	case "ssh":
		p.SSH.HostKeySHA256 = fingerprint
	default:
		return errors.New("solo los canales API y SSH usan huellas")
	}
	_, err := s.store.Put(p)
	return err
}
