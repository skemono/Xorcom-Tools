package pbx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// HostKeyCheck pins the server host key: unknown or changed keys fail with *UntrustedError.
func HostKeyCheck(pin string) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		fp := ssh.FingerprintSHA256(key)
		switch {
		case pin == "":
			return &UntrustedError{Fingerprint: fp}
		case pin != fp:
			return &UntrustedError{Fingerprint: fp, Changed: true}
		}
		return nil
	}
}

// SSHRun connects with the key file and/or password and runs one command, returning trimmed output.
func SSHRun(ctx context.Context, p Profile, password, keyPassphrase, cmd string) (string, error) {
	out, err := SSHExec(ctx, p, password, keyPassphrase, cmd, nil)
	return strings.TrimSpace(out), err
}

// sshDialTimeout bounds reaching the PBX: a host that never answers fails here, with the
// Spanish "sin respuesta" message, instead of after Windows' own 21 s connect timeout.
var sshDialTimeout = 10 * time.Second

// SSHExec runs one command with stdin (secrets and data travel there, never in the command line).
// It returns stdout as is; on failure the error carries the remote stderr.
func SSHExec(ctx context.Context, p Profile, password, keyPassphrase, cmd string, stdin io.Reader) (string, error) {
	var out bytes.Buffer
	err := SSHStream(ctx, p, password, keyPassphrase, cmd, stdin, &out)
	return out.String(), err
}

// SSHStream is SSHExec writing stdout to w as it arrives (progress of a long command).
func SSHStream(ctx context.Context, p Profile, password, keyPassphrase, cmd string, stdin io.Reader, w io.Writer) error {
	var auth []ssh.AuthMethod
	if p.SSH.KeyPath != "" {
		raw, err := os.ReadFile(p.SSH.KeyPath)
		if err != nil {
			return fmt.Errorf("no se pudo leer la llave SSH: %w", err)
		}
		var signer ssh.Signer
		if keyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(raw, []byte(keyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(raw)
		}
		if err != nil {
			return fmt.Errorf("llave SSH inválida: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if password != "" {
		answer := func(_, _ string, questions []string, _ []bool) ([]string, error) {
			out := make([]string, len(questions))
			for i := range out {
				out[i] = password
			}
			return out, nil
		}
		auth = append(auth, ssh.Password(password), ssh.KeyboardInteractive(answer))
	}
	if len(auth) == 0 {
		return errors.New("falta la contraseña o la llave SSH")
	}

	// Keep the pin verdict even if the ssh package wraps the callback error without %w.
	var untrusted *UntrustedError
	check := HostKeyCheck(p.SSH.HostKeySHA256)
	cfg := &ssh.ClientConfig{
		User: p.SSH.User,
		Auth: auth,
		HostKeyCallback: func(h string, a net.Addr, k ssh.PublicKey) error {
			err := check(h, a, k)
			errors.As(err, &untrusted)
			return err
		},
	}

	addr := net.JoinHostPort(p.Host, strconv.Itoa(p.SSH.Port))
	d := net.Dialer{Timeout: sshDialTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	if dl, ok := ctx.Deadline(); ok {
		conn.SetDeadline(dl)
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		if untrusted != nil {
			return untrusted
		}
		return err
	}
	client := ssh.NewClient(c, chans, reqs)
	defer client.Close()
	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	var stderr bytes.Buffer
	sess.Stdin, sess.Stdout, sess.Stderr = stdin, w, &stderr
	if err := sess.Run(cmd); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
