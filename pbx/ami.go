package pbx

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"strings"
)

// AMILogin dials the Asterisk Manager Interface, logs in and logs off. It returns the AMI banner.
// AMI on 5038 is plaintext: restrict it on the PBX with permit/deny to the technicians' subnet.
func AMILogin(ctx context.Context, addr, user, secret string) (string, error) {
	if strings.ContainsAny(user+secret, "\r\n") {
		return "", errors.New("usuario o secreto AMI inválido: contiene saltos de línea")
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		conn.SetDeadline(dl)
	}
	r := textproto.NewReader(bufio.NewReader(conn))
	banner, err := r.ReadLine()
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(banner, "Asterisk Call Manager") {
		return "", fmt.Errorf("el puerto no responde como AMI: %q", banner)
	}
	if _, err := fmt.Fprintf(conn, "Action: Login\r\nUsername: %s\r\nSecret: %s\r\nEvents: off\r\n\r\n", user, secret); err != nil {
		return "", err
	}
	for { // skip events until the Login response arrives
		h, err := r.ReadMIMEHeader()
		if err != nil {
			return "", err
		}
		switch h.Get("Response") {
		case "":
			continue
		case "Success":
			fmt.Fprint(conn, "Action: Logoff\r\n\r\n")
			return banner, nil
		default:
			return "", fmt.Errorf("AMI rechazó el inicio de sesión: %s", h.Get("Message"))
		}
	}
}
