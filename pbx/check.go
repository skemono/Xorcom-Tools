package pbx

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ChannelResult is one channel's outcome in a connection test.
type ChannelResult struct {
	Channel            string `json:"channel"` // "api", "ssh" or "ami"
	Skipped            bool   `json:"skipped"`
	OK                 bool   `json:"ok"`
	Message            string `json:"message"`
	Millis             int64  `json:"millis"`
	Fingerprint        string `json:"fingerprint"`
	FingerprintChanged bool   `json:"fingerprintChanged"`
}

const checkTimeout = 10 * time.Second

// Check runs every enabled channel concurrently; one failure never stops the others.
func Check(ctx context.Context, p Profile, s Secrets) []ChannelResult {
	probes := []struct {
		channel string
		enabled bool
		run     func(context.Context) (string, error)
	}{
		{"api", p.API.Enabled, func(ctx context.Context) (string, error) { return APICheck(ctx, p, s.API) }},
		{"ssh", p.SSH.Enabled, func(ctx context.Context) (string, error) {
			host, err := SSHRun(ctx, p, s.SSH, s.SSHKey, "uname -n")
			if err != nil {
				return "", err
			}
			return "sesión iniciada en " + host, nil
		}},
		{"ami", p.AMI.Enabled, func(ctx context.Context) (string, error) {
			banner, err := AMILogin(ctx, net.JoinHostPort(p.Host, strconv.Itoa(p.AMI.Port)), p.AMI.User, s.AMI)
			if err != nil {
				return "", err
			}
			return "sesión AMI aceptada (" + banner + ")", nil
		}},
	}
	results := make([]ChannelResult, len(probes))
	var wg sync.WaitGroup
	for i, pr := range probes {
		results[i] = ChannelResult{Channel: pr.channel, Skipped: !pr.enabled}
		if !pr.enabled {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(ctx, checkTimeout)
			defer cancel()
			start := time.Now()
			msg, err := pr.run(ctx)
			r := &results[i]
			r.Millis = time.Since(start).Milliseconds()
			if err != nil {
				r.Message = describe(err)
				var ue *UntrustedError
				if errors.As(err, &ue) {
					r.Fingerprint, r.FingerprintChanged = ue.Fingerprint, ue.Changed
				}
				return
			}
			r.OK, r.Message = true, msg
		}()
	}
	wg.Wait()
	return results
}

// describe turns channel errors into short Spanish messages.
// ponytail: refused/auth cases match on error text; switch to typed checks if Go's messages drift.
func describe(err error) string {
	var ue *UntrustedError
	var dns *net.DNSError
	var ne net.Error
	msg := err.Error()
	switch {
	case errors.As(err, &ue):
		return ue.Error()
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return "sin respuesta en 10 s (equipo apagado o firewall)"
	case errors.As(err, &dns):
		return "no se encontró el host " + dns.Name
	case strings.Contains(msg, "refused"):
		return "conexión rechazada: puerto cerrado o servicio apagado"
	case strings.Contains(msg, "unable to authenticate"):
		return "usuario o contraseña SSH incorrectos"
	}
	return msg
}
