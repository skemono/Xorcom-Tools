package main

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const repoSlug = "skemono/Xorcom-Tools"

type UpdateInfo struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
}

// UpdateService replaces the running exe with the latest GitHub release.
type UpdateService struct {
	mu     sync.Mutex
	latest *selfupdate.Release
}

func (u *UpdateService) Version() string { return version }

// Check never reports updates for local "dev" builds.
func (u *UpdateService) Check() (UpdateInfo, error) {
	info := UpdateInfo{Current: version}
	if version == "dev" {
		return info, nil
	}
	up, err := newUpdater()
	if err != nil {
		return info, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	rel, found, err := up.DetectLatest(ctx, selfupdate.ParseSlug(repoSlug))
	if err != nil || !found {
		return info, err
	}
	info.Latest = rel.Version()
	if rel.LessOrEqual(version) {
		return info, nil
	}
	u.mu.Lock()
	u.latest = rel
	u.mu.Unlock()
	info.Available = true
	return info, nil
}

// Apply downloads the release found by Check, verifies checksums.txt and swaps the exe.
func (u *UpdateService) Apply() error {
	u.mu.Lock()
	rel := u.latest
	u.mu.Unlock()
	if rel == nil {
		return errors.New("no hay una actualización pendiente")
	}
	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return err
	}
	up, err := newUpdater()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return up.UpdateTo(ctx, rel, exe)
}

// Restart launches the (new) exe and quits this process.
func (u *UpdateService) Restart() error {
	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return err
	}
	if err := exec.Command(exe).Start(); err != nil {
		return err
	}
	application.Get().Quit()
	return nil
}

func newUpdater() (*selfupdate.Updater, error) {
	// ponytail: checksums ship in the same release, so a compromised GitHub account can ship a bad build;
	// upgrade path: sign checksums.txt with an offline key and use go-selfupdate's ECDSA validator.
	return selfupdate.NewUpdater(selfupdate.Config{
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
	})
}
