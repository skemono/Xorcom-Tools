package pbx

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
)

// UntrustedError is a certificate or host key that is neither CA-verified nor pinned.
type UntrustedError struct {
	Fingerprint string
	Changed     bool // a different pin is stored: reinstalled box or interception
}

func (e *UntrustedError) Error() string {
	if e.Changed {
		return "la huella cambió: posible equipo reinstalado o interceptación"
	}
	return "huella desconocida: verifíquela y confíe en ella"
}

// CertFingerprint is the SHA-256 of a DER certificate as AB:CD:... (how browsers show it).
func CertFingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":")
}

// PinnedTLS accepts a chain that verifies against system roots for host, or a leaf matching pin.
func PinnedTLS(host, pin string) *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true, // verification happens in VerifyConnection below
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("el servidor no envió certificado")
			}
			leaf := cs.PeerCertificates[0]
			inter := x509.NewCertPool()
			for _, c := range cs.PeerCertificates[1:] {
				inter.AddCert(c)
			}
			if _, err := leaf.Verify(x509.VerifyOptions{DNSName: host, Intermediates: inter}); err == nil {
				return nil
			}
			fp := CertFingerprint(leaf.Raw)
			if pin == "" {
				return &UntrustedError{Fingerprint: fp}
			}
			if !strings.EqualFold(pin, fp) {
				return &UntrustedError{Fingerprint: fp, Changed: true}
			}
			return nil
		},
	}
}
