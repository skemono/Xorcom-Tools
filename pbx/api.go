package pbx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

// portal talks to the CompletePBX 5 portal the way its own JS does: a cookie jar for the "sid"
// session, AJAX headers, JSON answers; https base URLs go through PinnedTLS.
type portal struct {
	client *http.Client
	jar    http.CookieJar
	base   string
	url    *url.URL
}

type portalReply struct {
	State        string `json:"state"`
	Notification struct {
		Text string `json:"text"`
	} `json:"notification"`
}

func newPortal(p Profile) (*portal, error) {
	base := strings.TrimRight(p.APIBase(), "/")
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("URL de API inválida: %q", base)
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Transport: &http.Transport{TLSClientConfig: PinnedTLS(u.Hostname(), p.API.CertSHA256)}}
	return &portal{client: client, jar: jar, base: base, url: u}, nil
}

// ajax sends req with the portal's AJAX headers (without them it answers HTML) and decodes the JSON.
func (pt *portal) ajax(req *http.Request) (portalReply, error) {
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	resp, err := pt.client.Do(req)
	if err != nil {
		return portalReply{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return portalReply{}, fmt.Errorf("el servidor respondió %s", resp.Status)
	}
	var r portalReply
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return portalReply{}, fmt.Errorf("respuesta inesperada del portal (¿es una CompletePBX 5?): %v", err)
	}
	return r, nil
}

// login posts the portal's login form; success needs state "success" and a "sid" cookie.
func (pt *portal) login(ctx context.Context, user, password string) error {
	form := url.Values{"userid": {user}, "userpass": {password}, "baseurl": {pt.base}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pt.base+"/login", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r, err := pt.ajax(req)
	if err != nil {
		return err
	}
	if r.State != "success" || !hasCookie(pt.jar, pt.url, "sid") {
		msg := r.Notification.Text
		if msg == "" {
			msg = "usuario o contraseña incorrectos (en una PBX nueva, el admin del portal aún no tiene contraseña)"
		}
		return fmt.Errorf("el portal rechazó el inicio de sesión: %s", msg)
	}
	return nil
}

func portalUser(p Profile) string {
	if p.API.User == "" {
		return "admin"
	}
	return p.API.User
}

// APICheck verifies the portal: with a password it logs in; without one it only proves the base URL answers.
func APICheck(ctx context.Context, p Profile, password string) (string, error) {
	pt, err := newPortal(p)
	if err != nil {
		return "", err
	}
	defer pt.client.CloseIdleConnections()
	if password == "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pt.base+"/", nil)
		if err != nil {
			return "", err
		}
		resp, err := pt.client.Do(req)
		if err != nil {
			return "", err
		}
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			return "", fmt.Errorf("el servidor respondió %s", resp.Status)
		}
		return fmt.Sprintf("el portal responde (%s); sin contraseña guardada, no se probó el inicio de sesión", resp.Status), nil
	}
	if err := pt.login(ctx, portalUser(p), password); err != nil {
		return "", err
	}
	return "sesión iniciada en el portal como " + portalUser(p), nil
}

// PortalApplyChanges logs in and runs the portal's own Apply (GET /apply-changes). It reloads the PBX
// with ALL outstanding portal changes, so it is only ever called from an explicit user action.
func PortalApplyChanges(ctx context.Context, p Profile, password string) (string, error) {
	pt, err := newPortal(p)
	if err != nil {
		return "", err
	}
	defer pt.client.CloseIdleConnections()
	if err := pt.login(ctx, portalUser(p), password); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pt.base+"/apply-changes", nil)
	if err != nil {
		return "", err
	}
	r, err := pt.ajax(req)
	if err != nil {
		return "", err
	}
	if r.State != "success" {
		reason := r.Notification.Text
		if reason == "" {
			reason = r.State
		}
		return "", fmt.Errorf("el portal no aplicó los cambios: %s", reason)
	}
	if r.Notification.Text == "" {
		return "cambios aplicados en la PBX", nil
	}
	return r.Notification.Text, nil
}

func hasCookie(jar http.CookieJar, u *url.URL, name string) bool {
	for _, c := range jar.Cookies(u) {
		if c.Name == name {
			return true
		}
	}
	return false
}
