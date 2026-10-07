package pbx

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// fakeSSH serves exec requests: the handler gets the command and the full stdin and returns
// stdout, stderr and the exit code. Password "pw" is the only accepted credential.
func fakeSSH(t *testing.T, handle func(cmd string, stdin []byte) (string, string, int)) (string, string) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{PasswordCallback: func(_ ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
		if string(pw) == "pw" {
			return nil, nil
		}
		return nil, errors.New("denied")
	}}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go serveSSH(c, cfg, handle)
		}
	}()
	return ln.Addr().String(), ssh.FingerprintSHA256(signer.PublicKey())
}

func serveSSH(c net.Conn, cfg *ssh.ServerConfig, handle func(string, []byte) (string, string, int)) {
	_, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		if nc.ChannelType() != "session" {
			nc.Reject(ssh.UnknownChannelType, "session only")
			continue
		}
		ch, creqs, err := nc.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer ch.Close()
			for req := range creqs {
				if req.Type != "exec" {
					req.Reply(false, nil)
					continue
				}
				var p struct{ Command string }
				ssh.Unmarshal(req.Payload, &p)
				req.Reply(true, nil)
				in, _ := io.ReadAll(ch)
				out, errOut, code := handle(p.Command, in)
				io.WriteString(ch, out)
				io.WriteString(ch.Stderr(), errOut)
				ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{uint32(code)}))
				return
			}
		}()
	}
}

func sshProfile(t *testing.T, addr, fp string) Profile {
	t.Helper()
	host, port, _ := net.SplitHostPort(addr)
	n, _ := strconv.Atoi(port)
	return Profile{Host: host, SSH: SSHConfig{Enabled: true, Port: n, User: "root", HostKeySHA256: fp}}
}

func TestSSHExecStdinAndStderr(t *testing.T) {
	addr, fp := fakeSSH(t, func(cmd string, in []byte) (string, string, int) {
		if cmd == "fail" {
			return "", "ERROR 1146: tabla no existe\n", 1
		}
		return strings.ToUpper(string(in)), "", 0
	})
	p := sshProfile(t, addr, fp)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := SSHExec(ctx, p, "pw", "", "upper", strings.NewReader("hola\tmundo\n"))
	if err != nil || out != "HOLA\tMUNDO\n" {
		t.Fatalf("stdin must reach the command and stdout come back raw: %q %v", out, err)
	}
	if _, err := SSHExec(ctx, p, "pw", "", "fail", nil); err == nil || !strings.Contains(err.Error(), "ERROR 1146") {
		t.Fatalf("stderr must be in the error, got %v", err)
	}
	if out, err := SSHRun(ctx, p, "pw", "", "upper"); err != nil || out != "" {
		t.Fatalf("SSHRun with no stdin must still work: %q %v", out, err)
	}
}
