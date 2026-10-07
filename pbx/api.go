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

// APICheck verifies the CompletePBX 5 portal API. With a password it logs in the way the portal's
// own JS does (POST /login, AJAX headers, form with baseurl) and requires state "success" plus a
// "sid" session cookie. Without one it only proves the base URL answers. HTTPS goes through PinnedTLS.
func APICheck(ctx context.Context, p Profile, password string) (string, error) {
	base := strings.TrimRight(p.API.BaseURL, "/")
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("URL de API inválida: %q", p.API.BaseURL)
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Transport: &http.Transport{TLSClientConfig: PinnedTLS(u.Hostname(), p.API.CertSHA256)}}
	defer client.CloseIdleConnections()

	if password == "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/", nil)
		if err != nil {
			return "", err
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			return "", fmt.Errorf("el servidor respondió %s", resp.Status)
		}
		return fmt.Sprintf("el portal responde (%s); sin contraseña guardada, no se probó el inicio de sesión", resp.Status), nil
	}

	user := p.API.User
	if user == "" {
		user = "admin"
	}
	form := url.Values{"userid": {user}, "userpass": {password}, "baseurl": {base}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/login", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Requested-With", "XMLHttpRequest") // without it the portal answers with HTML
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return "", fmt.Errorf("el servidor respondió %s", resp.Status)
	}
	var body struct {
		State        string `json:"state"`
		Notification struct {
			Text string `json:"text"`
		} `json:"notification"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("respuesta inesperada del portal (¿es una CompletePBX 5?): %v", err)
	}
	if body.State != "success" || !hasCookie(jar, u, "sid") {
		msg := body.Notification.Text
		if msg == "" {
			msg = "usuario o contraseña incorrectos (en una PBX nueva, el admin del portal aún no tiene contraseña)"
		}
		return "", fmt.Errorf("el portal rechazó el inicio de sesión: %s", msg)
	}
	return "sesión iniciada en el portal como " + user, nil
}

func hasCookie(jar http.CookieJar, u *url.URL, name string) bool {
	for _, c := range jar.Cookies(u) {
		if c.Name == name {
			return true
		}
	}
	return false
}
