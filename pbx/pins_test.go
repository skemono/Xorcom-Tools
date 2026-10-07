package pbx

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
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

func TestParseBatch(t *testing.T) {
	// mysql --batch escapes a tab inside a value as \t, a backslash as \\ and a newline as \n.
	rows := parseBatch("id\tdescription\n7\tlista\\tuno\n8\tcon\\\\barra\\n\n")
	if len(rows) != 2 || rows[0]["id"] != "7" || rows[0]["description"] != "lista\tuno" || rows[1]["description"] != "con\\barra\n" {
		t.Fatalf("batch rows/unescape wrong: %#v", rows)
	}
	if parseBatch("") != nil {
		t.Fatal("no output means no rows")
	}
}

func TestMySQLRunsThroughStdin(t *testing.T) {
	var gotCmd, gotSQL string
	addr, fp := fakeSSH(t, func(cmd string, in []byte) (string, string, int) {
		gotCmd, gotSQL = cmd, string(in)
		return "n\n3\n", "", 0
	})
	rows, err := MySQL(context.Background(), sshProfile(t, addr, fp), Secrets{SSH: "pw"}, "SELECT COUNT(*) AS n FROM x;")
	if err != nil || len(rows) != 1 || rows[0]["n"] != "3" {
		t.Fatalf("rows %v err %v", rows, err)
	}
	if gotCmd != mysqlCmd || gotSQL != "SELECT COUNT(*) AS n FROM x;" {
		t.Fatalf("SQL must go on stdin with the fixed command; cmd=%q sql=%q", gotCmd, gotSQL)
	}
}

func TestParsePinCSVExcelQuirks(t *testing.T) {
	// Windows-1252 bytes for "José Peña" (0xE9, 0xF1), ; separator, CRLF, trailing empty cell, quoted separator, blank line.
	raw := []byte("PIN;Descripcion\r\n4321;Jos\xe9 Pe\xf1a;\r\n\r\n5*55;\"Perez; Juan\"\r\n0123;\r\n")
	info, rows, err := ParsePinCSV(raw, 7)
	if err != nil {
		t.Fatal(err)
	}
	if info.Separator != ";" || info.Encoding != "Windows-1252" || !info.Header || info.Columns != 2 || info.Rows != 3 {
		t.Fatalf("info %+v", info)
	}
	want := []PinRow{
		{Line: 2, PIN: "4321", Description: "Jose Pena", Filtered: true},
		{Line: 4, PIN: "5*55", Description: "Perez Juan", Filtered: true}, // portal: no ";" in descriptions
		{Line: 5, PIN: "0123", Description: ""},
	}
	for i, w := range want {
		if rows[i] != w {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], w)
		}
	}
}

func TestParsePinCSVThreeColumnsAndUTF8(t *testing.T) {
	raw := []byte("\xef\xbb\xbfpin_list_id,password,description\n7,4321,Dr. O\u2019Brien \u2013 Rayos X\n8,5555,Otro\n")
	info, rows, err := ParsePinCSV(raw, 7)
	if err != nil || info.Encoding != "UTF-8" || info.Separator != "," || info.Columns != 3 {
		t.Fatalf("info %+v err %v", info, err)
	}
	if rows[0].Description != "Dr OBrien - Rayos X" || rows[0].Status != "" {
		t.Errorf("typographic quote/dash become ASCII, then only portal-safe characters remain: %+v", rows[0])
	}
	if rows[1].Status != PinError || !strings.Contains(rows[1].Error, "no coincide") {
		t.Errorf("a pin_list_id of another list must be a row error: %+v", rows[1])
	}
}

func TestValidatePins(t *testing.T) {
	raw := []byte("12a4;Primera fila invalida\n*99;Asterisco\n4321;Uno\n4321;Repetido\n;Sin PIN\n777;Linea\u0001control\n888;Stra\u00dfe\n999;Mar\u00eca \u00c7elik\n")
	_, rows, err := ParsePinCSV(raw, 1)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Status != PinError {
		t.Errorf("a first row with digits is data, so '12a4' must be an error, not a skipped header: %+v", rows[0])
	}
	if rows[1].Status != "" || rows[1].PIN != "*99" {
		t.Errorf("'*99' is a valid PIN kept as text: %+v", rows[1])
	}
	if rows[2].Status != PinError || rows[3].Status != PinError || !strings.Contains(rows[3].Error, "repetido") {
		t.Errorf("both rows of a repeated PIN must be errors: %+v %+v", rows[2], rows[3])
	}
	if rows[4].Status != PinError || rows[5].Status != PinError || rows[6].Status != PinError || !strings.Contains(rows[6].Error, "ß") {
		t.Errorf("missing PIN, control chars and unmapped non-ASCII are errors: %+v %+v %+v", rows[4], rows[5], rows[6])
	}
	if rows[7].Status != "" || rows[7].Description != "Maria Celik" || !rows[7].Filtered {
		t.Errorf("grave accents and ç are filtered like the rest: %+v", rows[7])
	}
}

// The portal refuses to save a list whose entry description is not "alphanumeric with dash and
// underscore" (spaces accepted on the lab). Anything F-02 writes must stay editable in the portal.
func TestDescriptionsArePortalSafe(t *testing.T) {
	cases := map[string]string{
		"Perez; Juan":          "Perez Juan",
		"Dr. O'Brien":          "Dr OBrien",
		"Lab/Rayos-X_2":        "Lab Rayos-X_2",
		"  Ana ,  \"Gomez\"  ": "Ana Gomez",
		"123456_Ana_Lopez":     "123456_Ana_Lopez",
		"José (turno #3)!":     "Jose turno 3",
	}
	for in, want := range cases {
		got, _ := filterDescription(in)
		if got != want {
			t.Errorf("filterDescription(%q) = %q, want %q", in, got, want)
		}
		for _, r := range got {
			if !(r == ' ' || r == '-' || r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
				t.Errorf("%q kept %q, which the portal rejects", got, r)
			}
		}
	}
	if got, changed := filterDescription("Turno nocturno"); got != "Turno nocturno" || changed {
		t.Errorf("a portal-safe description is left alone: %q %v", got, changed)
	}
}

// The portal's Apply banner is the ombutel module's "reload" setting; a portal Apply pushes every
// pending change, so F-02 warns when someone else's changes are waiting.
func TestPortalPending(t *testing.T) {
	for value, want := range map[string]bool{"value\nyes\n": true, "value\nno\n": false, "": false} {
		var gotSQL string
		addr, fp := fakeSSH(t, func(_ string, in []byte) (string, string, int) {
			gotSQL = string(in)
			return value, "", 0
		})
		got, err := PortalPending(context.Background(), sshProfile(t, addr, fp), Secrets{SSH: "pw"})
		if err != nil || got != want {
			t.Errorf("output %q: pending = %v, %v; want %v", value, got, err, want)
		}
		if !strings.Contains(gotSQL, "m.name = 'ombutel'") || !strings.Contains(gotSQL, "s.name = 'reload'") {
			t.Fatalf("must read the ombutel module's reload flag, sent:\n%s", gotSQL)
		}
	}
}

// An unclosed quote swallows the following lines into one cell; those PINs must never vanish silently.
func TestParsePinCSVUnclosedQuoteIsAnError(t *testing.T) {
	_, rows, err := ParsePinCSV([]byte("1111;\"Ana\n2222;Bob\n3333;\"Carl\"\n"), 1)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		if r.Status == PinError && strings.Contains(r.Error, "varias líneas") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a cell spanning several lines must be a row error, got %+v", rows)
	}
}

// One row with an extra cell (a note in column C, an unquoted ; in a description) is that row's
// error; it must not turn the whole file into the 3-column format.
func TestParsePinCSVStrayCellIsARowError(t *testing.T) {
	info, rows, err := ParsePinCSV([]byte("PIN;Descripcion\n4321;Ana\n5555;Bob;nota al margen\n6666;Carla\n7777;\n"), 1)
	if err != nil || info.Columns != 2 {
		t.Fatalf("info %+v err %v", info, err)
	}
	if rows[0].Status != "" || rows[2].Status != "" || rows[3].Status != "" {
		t.Errorf("the other rows are unaffected: %+v", rows)
	}
	if rows[1].Status != PinError || !strings.Contains(rows[1].Error, "3 celdas") {
		t.Errorf("the stray row names its problem: %+v", rows[1])
	}
}

func TestParsePinCSVFileErrors(t *testing.T) {
	cases := map[string][]byte{
		"xlsx":       []byte("PK\x03\x04\x14\x00rest-of-zip"),
		"empty":      []byte("\r\n\r\n"),
		"one column": []byte("PIN\n4321\n5555\n"),
		"four cols":  []byte("a;b;c;d\n1;2;3;4\n"),
		"old xls":    []byte("\xD0\xCF\x11\xE0\xA1\xB1\x1A\xE1\x00\x00\x00\x00rest-of-ole2"),
		"utf-16":     []byte("\xFF\xFEP\x00I\x00N\x00;\x00D\x00\n\x004\x003\x002\x001\x00;\x00A\x00n\x00a\x00\n\x00"),
	}
	for name, raw := range cases {
		if _, _, err := ParsePinCSV(raw, 1); err == nil {
			t.Errorf("%s: want a file-level error", name)
		}
	}
	big := []byte(strings.Repeat("1;x\n", maxPinRows+1))
	if _, _, err := ParsePinCSV(big, 1); err == nil {
		t.Error("more than maxPinRows rows must be refused")
	}
}

func TestPlanPins(t *testing.T) {
	rows := []PinRow{
		{Line: 1, PIN: "1", Description: "a"},
		{Line: 2, PIN: "2", Description: "b"},
		{Line: 3, PIN: "x", Status: PinError},
		{Line: 4, PIN: "4"}, // no description
	}
	got := PlanPins(rows, map[string]string{"2": "ya"}, false)
	if got[0].Status != PinNew || got[1].Status != PinExists || got[2].Status != PinError || got[3].Status != PinNoDesc {
		t.Fatalf("plan without empty descriptions: %+v", got)
	}
	if got := PlanPins(rows, nil, true); got[3].Status != PinNew {
		t.Fatalf("with the toggle on, an empty description is loaded: %+v", got[3])
	}
}

func TestInsertSQLEscapes(t *testing.T) {
	sql := InsertSQL(7, []PinRow{
		{PIN: "4321", Description: `O'Brien \ Lab`, Status: PinNew},
		{PIN: "5555", Status: PinNew},
		{PIN: "6666", Description: "existe", Status: PinExists},
	})
	for _, want := range []string{
		"START TRANSACTION;",
		`SELECT 7, '4321', 'O''Brien \\ Lab' FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM ombu_pin_list_entries WHERE pin_list_id = 7 AND password = '4321');`,
		"SELECT 7, '5555', NULL FROM DUAL",
		"COMMIT;",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL missing %q:\n%s", want, sql)
		}
	}
	if strings.Contains(sql, "6666") {
		t.Error("existing PINs must not be inserted")
	}
}

func TestInsertSQLProgressMarkers(t *testing.T) {
	rows := make([]PinRow, 250)
	for i := range rows {
		rows[i] = PinRow{PIN: strconv.Itoa(1000 + i), Status: PinNew}
	}
	sql := InsertSQL(7, rows)
	at := -1
	for _, n := range []string{"100", "200", "250"} {
		i := strings.Index(sql, "SELECT "+n+" AS hecho;")
		if i < at {
			t.Fatalf("marker %s missing or out of order:\n%s", n, sql)
		}
		at = i
	}
	if at > strings.Index(sql, "COMMIT;") {
		t.Fatal("the last marker must come before COMMIT")
	}
	if c := strings.Count(InsertSQL(7, rows[:200]), "AS hecho"); c != 2 {
		t.Fatalf("200 rows need 2 markers, got %d", c)
	}
}

func TestApplyPinsReportsProgress(t *testing.T) {
	marker := regexp.MustCompile(`SELECT (\d+) AS hecho;`)
	addr, fp := fakeSSH(t, func(cmd string, in []byte) (string, string, int) {
		sql := string(in)
		if !strings.HasPrefix(sql, "START TRANSACTION") {
			return "pin\tdescription\n", "", 0
		}
		if !strings.Contains(cmd, "--unbuffered") {
			return "", "mysql would hold the counts until the end without --unbuffered", 1
		}
		var out strings.Builder
		for _, m := range marker.FindAllStringSubmatch(sql, -1) {
			out.WriteString("hecho\n" + m[1] + "\n")
		}
		return out.String(), "", 0
	})
	rows := make([]PinRow, 250)
	for i := range rows {
		rows[i] = PinRow{Line: i + 1, PIN: strconv.Itoa(1000 + i), Status: PinNew}
	}
	var got []int
	ApplyPins(context.Background(), sshProfile(t, addr, fp), Secrets{SSH: "pw"}, 7, rows, func(n int) { got = append(got, n) })
	if fmt.Sprint(got) != "[100 200 250]" {
		t.Fatalf("progress %v", got)
	}
}

func TestPinListsAndEntries(t *testing.T) {
	addr, fp := fakeSSH(t, func(_ string, in []byte) (string, string, int) {
		sql := string(in)
		switch {
		case strings.Contains(sql, "FROM ombu_pin_lists"):
			return "id\tdescription\tentries\n7\tGuardia\t2\n", "", 0
		case strings.Contains(sql, "pin_list_id = 7"):
			return "pin\tdescription\n4321\tJuan\n5555\t\n", "", 0
		}
		return "", "unexpected", 1
	})
	p, sec := sshProfile(t, addr, fp), Secrets{SSH: "pw"}
	lists, err := PinLists(context.Background(), p, sec)
	if err != nil || len(lists) != 1 || lists[0] != (PinList{ID: 7, Description: "Guardia", Entries: 2}) {
		t.Fatalf("lists %+v err %v", lists, err)
	}
	got, err := PinEntries(context.Background(), p, sec, 7)
	if err != nil || len(got) != 2 || got["4321"] != "Juan" || got["5555"] != "" {
		t.Fatalf("entries %v err %v", got, err)
	}
}

func TestApplyPins(t *testing.T) {
	var insert string
	addr, fp := fakeSSH(t, func(_ string, in []byte) (string, string, int) {
		sql := string(in)
		if strings.HasPrefix(sql, "START TRANSACTION") {
			insert = sql
			return "", "", 0
		}
		return "pin\tdescription\n4321\tO'Brien\n6666\tvieja\n", "", 0 // 5555 did not land
	})
	rows := []PinRow{
		{Line: 1, PIN: "4321", Description: "O'Brien", Status: PinNew},
		{Line: 2, PIN: "5555", Description: "Ana", Status: PinNew},
		{Line: 3, PIN: "6666", Status: PinExists},
		{Line: 4, PIN: "7777", Status: PinNoDesc},
	}
	out := ApplyPins(context.Background(), sshProfile(t, addr, fp), Secrets{SSH: "pw"}, 7, rows, func(int) {})
	if out[0].Status != PinApplied || out[1].Status != PinFailed || out[2].Status != PinSkipped || out[3].Status != PinSkipped {
		t.Fatalf("statuses %+v", out)
	}
	if out[2].Error != "ya existía" || out[3].Error != "sin descripción" {
		t.Fatalf("skipped lines must say why: %q %q", out[2].Error, out[3].Error)
	}
	if !strings.Contains(insert, "'O''Brien'") || strings.Contains(insert, "6666") || strings.Contains(insert, "7777") {
		t.Fatalf("insert sent on stdin wrong:\n%s", insert)
	}

	failAddr, failFP := fakeSSH(t, func(string, []byte) (string, string, int) {
		return "", "ERROR 1452 (23000) at line 2: Cannot add or update a child row", 1
	})
	out = ApplyPins(context.Background(), sshProfile(t, failAddr, failFP), Secrets{SSH: "pw"}, 7, rows, func(int) {})
	if out[0].Status != PinFailed || !strings.Contains(out[0].Error, "1452") || out[2].Status != PinSkipped {
		t.Fatalf("a failed transaction fails every new row with the reason: %+v", out)
	}
}
