# F-02 PINes masivos Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A second tool, F-02, that loads a CSV of PINs into an existing PIN list of the active CompletePBX 5, add-only, with a canary preview, a single SQL transaction over SSH stdin, and per-line stamps from a read-back.

**Architecture:** Pure CSV/validation/plan/SQL functions plus the SSH+mysql runner live in `pbx` and are unit-tested against an in-process fake SSH server. A thin bound `PinService` glues them to the active profile and keeps the last preview so Aplicar writes exactly what was previewed. The F-02 Svelte form extends the established Boleta world.

**Tech Stack:** Go 1.27, Wails v3.0.0-beta.28, Svelte 5 + TS, `golang.org/x/crypto/ssh` (client and, in tests, server), `encoding/csv`.

**Spec:** [docs/superpowers/specs/2026-10-07-f02-pines-masivos-design.md](../specs/2026-10-07-f02-pines-masivos-design.md) · prior spec: [2026-10-07-utilidades-xorcom-design.md](../specs/2026-10-07-utilidades-xorcom-design.md) · [DESIGN.md](../../../DESIGN.md)

## Global Constraints

- ROOT `"/c/Users/lordk/OneDrive/Documents/Documents/Documents/IASA/IGSS/Utilidades XORCOM"`; prefix Go/Wails commands with `export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH" &&`.
- Verify with `go vet . ./pbx/...`, `go build .`, `go test . ./pbx/...` (never `./...`: the template's `build/ios` package fails on Windows). Bindings: `wails3 generate bindings -clean=true -ts -i`.
- Credentials, the lab IP and the lab host-key fingerprint never go into repo files, tests, commits or memory. Lab access in commands only through env vars (`LAB_HOST`, `LAB_SSH`, `LAB_WEB`, `LAB_FP`) typed in the command, from the conversation.
- Every write to the lab box (Task 1 test PIN, Task 8 real load, all cleanups) needs the user's explicit go-ahead in that moment. Reads are fine.
- Add-only: F-02 never updates or deletes `ombu_pin_list_entries` rows (cleanup of test rows in Tasks 1/8 is a manual, user-approved step, not app code).
- PIN = digits and `*`; description = printable ASCII after the filter (ñ, every accented Latin vowel, ç, ý/ÿ and typographic quotes/dashes mapped); empty description → status `sindesc` (skipped) unless "Incluir PINes sin descripción" is on, then SQL `NULL`.
- `docs/test_data/` holds real people's names and PINs: git-ignored, never committed, never printed. Inspect it only through aggregate counts or line numbers.
- The portal's `GET /apply-changes` reloads the PBX with all outstanding portal changes: only ever called from an explicit, two-step user action, and only after the user's go-ahead on the lab.
- mysql runs as `mysql --batch --default-character-set=utf8 ombutel` with SQL on stdin; never put data in the command line; one `SELECT` per call (batch output of several result sets is not separable).
- Spanish for every user-facing string; English for code.
- Commits end with:
  `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>` and `Claude-Session: https://claude.ai/code/session_0189kqTXMUpmvoaPH1rgHBFi`.
- Never `git add -A` blindly: stage named paths (the PDF lesson).

## Review Focus

1. **Excel quirks in real CSVs:** trailing empty cells (`4321;Juan;`), a description with the separator inside quotes (`"Perez, Juan"`), CRLF line ends, a blank line in the middle: must land in the right fields, no row dropped. → `TestParsePinCSVExcelQuirks` (Task 4).
2. **Apostrophes and backslashes in descriptions** (`O'Brien`, `Lab\Rayos X`): stored exactly, never breaking or injecting SQL. → `TestInsertSQLEscapes` (Task 4) and `TestApplyPins` stdin assertion (Task 5).
3. **Leading zeros and `*` in PINs** (`0123`, `*99`): kept as text, never numeric-parsed. → `TestValidatePins` (Task 4).
4. **Active PBX or list switched between Vista previa and Aplicar:** Aplicar must refuse instead of writing the old preview to the new target. → `TestPinPlanCheck` (Task 6).
5. **The wrong file picked** (an `.xlsx` renamed or chosen, an empty file, a one-column file): a clear file-level Spanish error, no garbage preview. → `TestParsePinCSVFileErrors` (Task 4).

---

## File Structure

| Path | Responsibility | Task |
|---|---|---|
| `pbx/ssh.go` | add `SSHExec` (stdin, separate stderr); `SSHRun` delegates to it | 2 |
| `pbx/mysql.go` | `MySQL` runner over `SSHExec`, `parseBatch`, `unescapeBatch` | 3 |
| `pbx/pins.go` | PIN types/constants, CSV decode + parse + validate, `PlanPins`, `InsertSQL`, `PinLists`, `PinEntries`, `ApplyPins` | 4, 5 |
| `pbx/check.go` | rename `describe` → exported `Describe` | 5 |
| `pbx/pins_test.go` | fake SSH server fixture + tests for Tasks 2–5 | 2–5 |
| `profiles.go` | unexported `active()` helper | 6 |
| `pins.go` | bound `PinService` (`Lists`, `PickCSV`, `Preview`, `Apply`), `pinPlan.check` | 6 |
| `pins_test.go` (package main) | `TestPinPlanCheck` | 6 |
| `main.go` | register `PinService` | 6 |
| `frontend/src/modules/Pines.svelte`, `frontend/src/modules.ts`, `frontend/src/app.css` | F-02 form, registry line, `.stamp.mini`, `.mark.struck` | 7 |
| `DESIGN.md` | canary preview copy moves from "not yet built" to built | 7 |

---

### Task 1: Lab verification of the Apply question and the portal re-save

No repo code. Produces two rulings consumed by Task 7: `NEEDS_PORTAL_APPLY` and `RESAVE_LOSES_DESCRIPTIONS`.

**Interfaces:**
- Consumes: the scratchpad probe program from the brainstorming session (`<scratchpad>/probe/probe.exe`, runs one SSH command with the app's `pbx.SSHRun`; env `LAB_HOST`, `LAB_SSH`, `LAB_FP`). If it is gone, recreate it: a `main` that builds a `pbx.Profile{Host, SSH{Enabled, Port 22, User root, HostKeySHA256: LAB_FP}}` and prints `pbx.SSHRun(ctx, p, LAB_SSH, "", os.Args[1])`, module `probe` with `replace github.com/skemono/Xorcom-Tools => "<ROOT path>"`.
- Produces: ledger lines `Task 1: Ruling: NEEDS_PORTAL_APPLY = <true|false> — <evidence>` and `Task 1: Ruling: RESAVE_LOSES_DESCRIPTIONS = <true|false> — <evidence>`.

- [ ] **Step 1: Ask the user to prepare the lab** (in chat): the PIN list `PruebaIGSS` already exists (created by the user). Ask them to assign it to an outbound route and press Apply in the portal, and which placeholder PIN they typed when creating it (below: `<PH>`). Wait for "listo".

- [ ] **Step 2: Baseline (read-only)**

```bash
cd "<scratchpad>/probe" && LAB_HOST=… LAB_SSH=… LAB_FP=… ./probe.exe 'mysql --batch ombutel -e "SELECT l.pin_list_id, l.description, e.password, e.description FROM ombu_pin_lists l LEFT JOIN ombu_pin_list_entries e USING (pin_list_id) WHERE l.description = '"'"'PruebaIGSS'"'"';"; echo "== astdb:"; asterisk -rx "database show" | grep -i -e <PH> -e pin | head; echo "== conf:"; grep -rln <PH> /etc/asterisk 2>/dev/null'
```
Expected: the list id, the placeholder row; note whether `<PH>` appears in AstDB or `/etc/asterisk` (this tells where the portal's Apply puts PINs).

- [ ] **Step 3: One test insert (ask the user first)**: "¿Inserto el PIN de prueba 999002 en PruebaIGSS?" On yes, with `<ID>` from Step 2:

```bash
./probe.exe 'mysql ombutel -e "INSERT INTO ombu_pin_list_entries (pin_list_id, password, description) VALUES (<ID>, '"'"'999002'"'"', '"'"'PRUEBA F02'"'"');"; asterisk -rx "database show" | grep -c 999002; grep -rln 999002 /etc/asterisk 2>/dev/null | head -3'
```

- [ ] **Step 4: Decide `NEEDS_PORTAL_APPLY`**
  - If Step 2 showed `<PH>` in AstDB/conf and Step 3 shows `999002` absent there → ask the user to press Apply in the portal, re-run the Step 3 greps: present now → `NEEDS_PORTAL_APPLY = true`.
  - If Step 2 showed the placeholder nowhere outside MySQL (Asterisk reads MySQL live) → ask the user to dial through the outbound route with PIN `999002` without pressing Apply: works → `false`; rejected → `true`.
  - Ledger the ruling with the evidence.

- [ ] **Step 5: Portal re-save check (ask the user)**: "Abra PruebaIGSS en el portal y guárdela sin cambios." Then re-run the Step 2 SELECT: if the `PRUEBA F02` description of `999002` is now empty/NULL or the row id changed → `RESAVE_LOSES_DESCRIPTIONS = true`, else `false`. Ledger it.

- [ ] **Step 6: Cleanup (ask the user first)**: delete the test row only:

```bash
./probe.exe 'mysql ombutel -e "DELETE FROM ombu_pin_list_entries WHERE pin_list_id = <ID> AND password = '"'"'999002'"'"';"'
```
Keep `PruebaIGSS` itself for Task 8 (it holds the placeholder `<PH>`).

---

### Task 2: `SSHExec` with stdin and separate stderr

**Files:**
- Modify: `pbx/ssh.go`
- Create: `pbx/pins_test.go` (fixture + this task's tests)

**Interfaces:**
- Consumes: `HostKeyCheck`, `UntrustedError` (existing)
- Produces: `func SSHExec(ctx context.Context, p Profile, password, keyPassphrase, cmd string, stdin io.Reader) (string, error)` — returns raw stdout; on failure the error wraps the cause and appends trimmed stderr. `SSHRun` keeps its signature and trims stdout. Test fixture `fakeSSH(t, handler) (addr, fingerprint string)` and `sshProfile(t, addr, fp) Profile`.

- [ ] **Step 1: Write the fixture and failing tests** — create `pbx/pins_test.go`:

```go
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
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./pbx/... -run TestSSHExec -count=1`
Expected: FAIL, `undefined: SSHExec`.

- [ ] **Step 3: Implement in `pbx/ssh.go`** — replace the body of `SSHRun` from `client := ssh.NewClient(...)` onward by moving it into `SSHExec`. Final shape of the two functions:

```go
// SSHRun connects with the key file and/or password and runs one command, returning trimmed output.
func SSHRun(ctx context.Context, p Profile, password, keyPassphrase, cmd string) (string, error) {
	out, err := SSHExec(ctx, p, password, keyPassphrase, cmd, nil)
	return strings.TrimSpace(out), err
}

// SSHExec runs one command with stdin (secrets and data travel here, never in the command line).
// It returns stdout as is; on failure the error carries the remote stderr.
func SSHExec(ctx context.Context, p Profile, password, keyPassphrase, cmd string, stdin io.Reader) (string, error) {
	// … the auth, HostKeyCallback, dial and NewClientConn code that SSHRun had, unchanged …
	client := ssh.NewClient(c, chans, reqs)
	defer client.Close()
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	var stdout, stderr bytes.Buffer
	sess.Stdin, sess.Stdout, sess.Stderr = stdin, &stdout, &stderr
	if err := sess.Run(cmd); err != nil {
		return stdout.String(), fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
```
Add `"bytes"` and `"io"` to the imports. Move (not copy) the auth/dial block so it exists once.

- [ ] **Step 4: Run all pbx tests**

Run: `go test ./pbx/... -count=1 -race`
Expected: PASS (new test plus every existing one, including `TestCheckSkipsDisabledAndIsolatesFailures`).

- [ ] **Step 5: Commit**

```bash
git add pbx/ssh.go pbx/pins_test.go && git commit -m "feat(pbx): SSHExec with stdin and remote stderr, tested on a fake SSH server

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_0189kqTXMUpmvoaPH1rgHBFi"
```

---

### Task 3: MySQL runner and batch-output parser

**Files:**
- Create: `pbx/mysql.go`
- Modify: `pbx/pins_test.go` (append)

**Interfaces:**
- Consumes: `SSHExec` (Task 2), `Secrets` (existing)
- Produces: `const mysqlCmd = "mysql --batch --default-character-set=utf8 ombutel"`; `func MySQL(ctx context.Context, p Profile, sec Secrets, sql string) ([]map[string]string, error)` (rows keyed by column name; nil for statements without a result set); `func parseBatch(out string) []map[string]string`; `func unescapeBatch(s string) string`.

- [ ] **Step 1: Write the failing tests** — append to `pbx/pins_test.go`:

```go
func TestParseBatch(t *testing.T) {
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
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./pbx/... -run 'TestParseBatch|TestMySQL' -count=1`
Expected: FAIL, `undefined: parseBatch`.

- [ ] **Step 3: Implement `pbx/mysql.go`**

```go
package pbx

import (
	"context"
	"fmt"
	"strings"
)

// mysqlCmd reaches the CompletePBX database as root over the socket (no DB password).
const mysqlCmd = "mysql --batch --default-character-set=utf8 ombutel"

// MySQL sends sql on stdin and returns the rows of its single result set by column name.
// Keep one SELECT per call: --batch prints consecutive result sets with no separator.
func MySQL(ctx context.Context, p Profile, sec Secrets, sql string) ([]map[string]string, error) {
	out, err := SSHExec(ctx, p, sec.SSH, sec.SSHKey, mysqlCmd, strings.NewReader(sql))
	if err != nil {
		return nil, fmt.Errorf("MySQL: %w", err)
	}
	return parseBatch(out), nil
}

// parseBatch reads mysql --batch output: a tab-separated header line, then one line per row.
func parseBatch(out string) []map[string]string {
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return nil
	}
	lines := strings.Split(out, "\n")
	cols := strings.Split(lines[0], "\t")
	rows := make([]map[string]string, 0, len(lines)-1)
	for _, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		row := make(map[string]string, len(cols))
		for i, c := range cols {
			if i < len(fields) {
				row[c] = unescapeBatch(fields[i])
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// unescapeBatch undoes mysql --batch escaping: \t \n \\ \0.
func unescapeBatch(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i == len(s)-1 {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 't':
			b.WriteByte('\t')
		case 'n':
			b.WriteByte('\n')
		case '0':
			b.WriteByte(0)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./pbx/... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pbx/mysql.go pbx/pins_test.go && git commit -m "feat(pbx): mysql batch runner over SSH stdin

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_0189kqTXMUpmvoaPH1rgHBFi"
```

---

### Task 4: CSV decode, parse, validate, plan and SQL (pure)

**Files:**
- Create: `pbx/pins.go`
- Modify: `pbx/pins_test.go` (append)

**Interfaces:**
- Produces:
  - status constants `PinNew="nuevo"`, `PinExists="existe"`, `PinError="error"`, `PinNoDesc="sindesc"`, `PinApplied="aplicado"`, `PinSkipped="omitido"`, `PinFailed="fallo"`
  - `type PinRow struct{ Line int; PIN, Description string; Filtered bool; Status, Error string }` (json `line, pin, description, filtered, status, error`)
  - `type CSVInfo struct{ Separator, Encoding string; Header bool; Columns, Rows int }` (json `separator, encoding, header, columns, rows`)
  - `func ParsePinCSV(raw []byte, listID int) (CSVInfo, []PinRow, error)` — error = file-level problem (Spanish); row problems are `Status: PinError` rows
  - `func PlanPins(rows []PinRow, existing map[string]string, includeEmpty bool) []PinRow` — non-error rows become `PinExists`, `PinNoDesc` (empty description, unless includeEmpty) or `PinNew`
  - `func InsertSQL(listID int, rows []PinRow) string` — transaction of `INSERT … SELECT … WHERE NOT EXISTS` for `PinNew` rows only
  - `func filterDescription(s string) (string, bool)`

- [ ] **Step 1: Write the failing tests** — append to `pbx/pins_test.go`:

```go
func rowsByStatus(rows []PinRow) map[string]int {
	m := map[string]int{}
	for _, r := range rows {
		m[r.Status]++
	}
	return m
}

func TestParsePinCSVExcelQuirks(t *testing.T) {
	// Windows-1252 bytes for "José" (0xE9), ; separator, CRLF, trailing empty cell, quoted separator, blank line.
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
		{Line: 4, PIN: "5*55", Description: "Perez; Juan"},
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
	if rows[0].Description != "Dr. O'Brien - Rayos X" || rows[0].Status != "" {
		t.Errorf("typographic quote/dash must become ASCII: %+v", rows[0])
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

func TestParsePinCSVFileErrors(t *testing.T) {
	cases := map[string][]byte{
		"xlsx":       []byte("PK\x03\x04\x14\x00rest-of-zip"),
		"empty":      []byte("\r\n\r\n"),
		"one column": []byte("PIN\n4321\n5555\n"),
		"four cols":  []byte("a;b;c;d\n1;2;3;4\n"),
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
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./pbx/... -run 'TestParsePin|TestValidatePins|TestPlanPins|TestInsertSQL' -count=1`
Expected: FAIL, `undefined: ParsePinCSV`.

- [ ] **Step 3: Implement `pbx/pins.go`**

```go
package pbx

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Row statuses: before Aplicar (nuevo/existe/error) and after (aplicado/omitido/fallo).
const (
	PinNew     = "nuevo"
	PinExists  = "existe"
	PinNoDesc  = "sindesc" // empty description, skipped unless the user includes them
	PinError   = "error"
	PinApplied = "aplicado"
	PinSkipped = "omitido"
	PinFailed  = "fallo"
)

const maxPinRows = 10000

// PinRow is one CSV line on its way to ombu_pin_list_entries.
type PinRow struct {
	Line        int    `json:"line"` // line number in the file, as the user sees it
	PIN         string `json:"pin"`
	Description string `json:"description"` // after filterDescription
	Filtered    bool   `json:"filtered"`    // ñ, accents or typographic characters were replaced
	Status      string `json:"status"`
	Error       string `json:"error"`
}

// CSVInfo is what F-02 detected about the file, shown to the user.
type CSVInfo struct {
	Separator string `json:"separator"` // ";", "," or "tab"
	Encoding  string `json:"encoding"`  // "UTF-8" or "Windows-1252"
	Header    bool   `json:"header"`
	Columns   int    `json:"columns"` // 2 (PIN, descripción) or 3 (pin_list_id, PIN, descripción)
	Rows      int    `json:"rows"`
}

// ParsePinCSV decodes and validates a PIN file for listID. A returned error is a file-level problem;
// row problems come back as PinError rows so the preview can show every line.
func ParsePinCSV(raw []byte, listID int) (CSVInfo, []PinRow, error) {
	var info CSVInfo
	if bytes.HasPrefix(raw, []byte("PK\x03\x04")) {
		return info, nil, errors.New("parece un archivo de Excel (.xlsx): ábralo en Excel y guárdelo como CSV")
	}
	text, enc := decodeText(raw)
	info.Encoding = enc
	sep, name := detectSeparator(text)
	info.Separator = name

	r := csv.NewReader(strings.NewReader(text))
	r.Comma, r.FieldsPerRecord, r.LazyQuotes, r.ReuseRecord = sep, -1, true, false
	type rec struct {
		line   int
		fields []string
	}
	var recs []rec
	for {
		fields, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return info, nil, fmt.Errorf("no se pudo leer el CSV: %v", err)
		}
		line, _ := r.FieldPos(0)
		for len(fields) > 0 && strings.TrimSpace(fields[len(fields)-1]) == "" { // Excel's trailing empty cells
			fields = fields[:len(fields)-1]
		}
		if len(fields) == 0 {
			continue
		}
		recs = append(recs, rec{line, fields})
		if len(recs) > maxPinRows+1 {
			return info, nil, fmt.Errorf("el archivo tiene más de %d filas", maxPinRows)
		}
	}
	if len(recs) == 0 {
		return info, nil, errors.New("el archivo está vacío")
	}
	for _, rc := range recs {
		info.Columns = max(info.Columns, len(rc.fields))
	}
	if info.Columns < 2 || info.Columns > 3 {
		return info, nil, fmt.Errorf("el archivo tiene %d columna(s); se esperan 2 (PIN, descripción) o 3 (pin_list_id, PIN, descripción)", info.Columns)
	}
	pinCol := info.Columns - 2 // 0 for 2 columns, 1 for 3
	field := func(f []string, i int) string {
		if i < len(f) {
			return strings.TrimSpace(f[i])
		}
		return ""
	}
	if !strings.ContainsAny(field(recs[0].fields, pinCol), "0123456789") {
		info.Header, recs = true, recs[1:]
	}
	if len(recs) > maxPinRows {
		return info, nil, fmt.Errorf("el archivo tiene más de %d filas", maxPinRows)
	}
	info.Rows = len(recs)

	rows := make([]PinRow, len(recs))
	first := map[string]int{} // PIN -> index of its first row
	for i, rc := range recs {
		row := PinRow{Line: rc.line, PIN: field(rc.fields, pinCol)}
		row.Description, row.Filtered = filterDescription(field(rc.fields, pinCol+1))
		switch {
		case info.Columns == 3 && field(rc.fields, 0) != strconv.Itoa(listID):
			row.Error = fmt.Sprintf("pin_list_id «%s» no coincide con la lista elegida (%d)", field(rc.fields, 0), listID)
		case row.PIN == "":
			row.Error = "falta el PIN"
		case strings.Trim(row.PIN, "0123456789*") != "":
			row.Error = fmt.Sprintf("PIN inválido «%s»: solo números y *", row.PIN)
		case len(row.PIN) > 255:
			row.Error = "PIN de más de 255 caracteres"
		case len(row.Description) > 255:
			row.Error = "descripción de más de 255 caracteres"
		default:
			if bad := firstNonASCII(row.Description); bad != "" {
				row.Error = fmt.Sprintf("carácter no permitido «%s» en la descripción", bad)
			}
		}
		if row.Error == "" {
			if j, dup := first[row.PIN]; dup {
				row.Error = fmt.Sprintf("PIN repetido (líneas %d y %d)", rows[j].Line, row.Line)
				rows[j].Error, rows[j].Status = row.Error, PinError
			} else {
				first[row.PIN] = i
			}
		}
		if row.Error != "" {
			row.Status = PinError
		}
		rows[i] = row
	}
	return info, rows, nil
}

// PlanPins marks every valid row as already in the list, without description (skipped unless
// includeEmpty) or new. existing maps PIN -> description.
func PlanPins(rows []PinRow, existing map[string]string, includeEmpty bool) []PinRow {
	out := append([]PinRow(nil), rows...)
	for i := range out {
		if out[i].Status == PinError {
			continue
		}
		_, inList := existing[out[i].PIN]
		switch {
		case inList:
			out[i].Status = PinExists
		case out[i].Description == "" && !includeEmpty:
			out[i].Status = PinNoDesc
		default:
			out[i].Status = PinNew
		}
	}
	return out
}

// InsertSQL is one transaction; NOT EXISTS keeps it add-only even if a PIN appeared after the preview.
func InsertSQL(listID int, rows []PinRow) string {
	var b strings.Builder
	b.WriteString("START TRANSACTION;\n")
	for _, r := range rows {
		if r.Status != PinNew {
			continue
		}
		desc := "NULL"
		if r.Description != "" {
			desc = sqlString(r.Description)
		}
		fmt.Fprintf(&b, "INSERT INTO ombu_pin_list_entries (pin_list_id, password, description) SELECT %d, %s, %s FROM DUAL WHERE NOT EXISTS (SELECT 1 FROM ombu_pin_list_entries WHERE pin_list_id = %d AND password = %s);\n",
			listID, sqlString(r.PIN), desc, listID, sqlString(r.PIN))
	}
	b.WriteString("COMMIT;\n")
	return b.String()
}

var sqlEscaper = strings.NewReplacer(`\`, `\\`, `'`, `''`)

func sqlString(s string) string { return "'" + sqlEscaper.Replace(s) + "'" }

// Descriptions are stored without ñ or any accent (user rule; the real list has grave accents too)
// and without Excel's typographic characters.
var descFilter = strings.NewReplacer(
	"à", "a", "á", "a", "â", "a", "ã", "a", "ä", "a", "å", "a",
	"è", "e", "é", "e", "ê", "e", "ë", "e",
	"ì", "i", "í", "i", "î", "i", "ï", "i",
	"ò", "o", "ó", "o", "ô", "o", "õ", "o", "ö", "o",
	"ù", "u", "ú", "u", "û", "u", "ü", "u",
	"ý", "y", "ÿ", "y", "ñ", "n", "ç", "c",
	"À", "A", "Á", "A", "Â", "A", "Ã", "A", "Ä", "A", "Å", "A",
	"È", "E", "É", "E", "Ê", "E", "Ë", "E",
	"Ì", "I", "Í", "I", "Î", "I", "Ï", "I",
	"Ò", "O", "Ó", "O", "Ô", "O", "Õ", "O", "Ö", "O",
	"Ù", "U", "Ú", "U", "Û", "U", "Ü", "U",
	"Ý", "Y", "Ñ", "N", "Ç", "C",
	"\u2018", "'", "\u2019", "'", "\u201C", `"`, "\u201D", `"`,
	"\u2013", "-", "\u2014", "-", "\u2026", "...", "\u00A0", " ",
)

func filterDescription(s string) (string, bool) {
	f := strings.TrimSpace(descFilter.Replace(s))
	return f, f != strings.TrimSpace(s)
}

// firstNonASCII returns the first character outside printable ASCII, or "".
func firstNonASCII(s string) string {
	for _, r := range s {
		if r < 0x20 || r > 0x7E {
			if r < 0x20 {
				return fmt.Sprintf("U+%04X", r)
			}
			return string(r)
		}
	}
	return ""
}

// decodeText: UTF-8 (BOM stripped) when valid, otherwise Windows-1252 (Excel's "CSV" on Spanish Windows).
func decodeText(raw []byte) (string, string) {
	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		return string(raw[3:]), "UTF-8"
	}
	if utf8.Valid(raw) {
		return string(raw), "UTF-8"
	}
	var b strings.Builder
	for _, c := range raw {
		switch {
		case c < 0x80:
			b.WriteByte(c)
		case c < 0xA0:
			b.WriteRune(cp1252[c-0x80])
		default:
			b.WriteRune(rune(c)) // 0xA0-0xFF match Latin-1
		}
	}
	return b.String(), "Windows-1252"
}

var cp1252 = [32]rune{
	0x20AC, 0xFFFD, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0xFFFD, 0x017D, 0xFFFD,
	0xFFFD, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0xFFFD, 0x017E, 0x0178,
}

// detectSeparator picks ;, , or tab by count outside quotes on the first non-empty line.
func detectSeparator(text string) (rune, string) {
	line := ""
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) != "" {
			line = l
			break
		}
	}
	counts := map[rune]int{}
	inQuotes := false
	for _, r := range line {
		switch {
		case r == '"':
			inQuotes = !inQuotes
		case !inQuotes && (r == ';' || r == ',' || r == '\t'):
			counts[r]++
		}
	}
	best := ','
	for _, r := range []rune{';', '\t'} {
		if counts[r] > counts[best] {
			best = r
		}
	}
	if best == '\t' {
		return best, "tab"
	}
	return best, string(best)
}
```

`FieldPos(0)` reports the 1-based physical line of the record's first field (blank lines count), which is what the user sees in Excel/Notepad.

- [ ] **Step 4: Run tests**

Run: `go test ./pbx/... -count=1`
Expected: PASS. If `TestParsePinCSVExcelQuirks` reports line numbers off by the blank line, the `FieldPos` value is the source of truth (it counts physical lines) and the expected `Line` values in the test are 2, 4 and 5 for that input.

- [ ] **Step 5: Commit**

```bash
git add pbx/pins.go pbx/pins_test.go && git commit -m "feat(pbx): PIN CSV decode/validate/plan and add-only insert SQL

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_0189kqTXMUpmvoaPH1rgHBFi"
```

---

### Task 5: Read lists/entries, apply with read-back, portal apply-changes

**Files:**
- Modify: `pbx/pins.go` (append), `pbx/api.go` (extract the portal session, add `PortalApplyChanges`), `pbx/check.go` (rename `describe` → `Describe`), `pbx/pbx_test.go` (`describe(` → `Describe(` in `TestDescribe`; `/apply-changes` route in `fakePortal`; `TestPortalApplyChanges`), `pbx/pins_test.go` (append)

**Interfaces:**
- Consumes: `MySQL` (T3), `InsertSQL`, statuses (T4)
- Produces:
  - `type PinList struct{ ID int; Description string; Entries int }` (json `id, description, entries`)
  - `func PinLists(ctx, p, sec) ([]PinList, error)`
  - `func PinEntries(ctx, p, sec, listID int) (map[string]string, error)` (PIN → description, `IFNULL` → "")
  - `func ApplyPins(ctx, p, sec, listID int, rows []PinRow) []PinRow` (PinNew → aplicado/fallo by read-back; PinExists → omitido "ya existía"; PinNoDesc → omitido "sin descripción")
  - `func PortalApplyChanges(ctx context.Context, p Profile, password string) (string, error)` (login + `GET /apply-changes`; returns the portal's notification text)
  - `func Describe(err error) string` (exported rename)
  - `APICheck` keeps its signature and behavior (its tests stay green)

- [ ] **Step 1: Write the failing tests** — append to `pbx/pins_test.go`:

```go
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
	out := ApplyPins(context.Background(), sshProfile(t, addr, fp), Secrets{SSH: "pw"}, 7, rows)
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
	out = ApplyPins(context.Background(), sshProfile(t, failAddr, failFP), Secrets{SSH: "pw"}, 7, rows)
	if out[0].Status != PinFailed || !strings.Contains(out[0].Error, "1452") || out[2].Status != PinSkipped {
		t.Fatalf("a failed transaction fails every new row with the reason: %+v", out)
	}
}
```

Then in `pbx/pbx_test.go`: add this route at the top of `fakePortal`'s handler (before the `/login` check), and the test below it:

```go
		if r.URL.Path == "/apply-changes" {
			if c, err := r.Cookie("sid"); err != nil || c.Value != "abc" || r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
				w.Write([]byte("<html>login page</html>"))
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"state": "success", "action": "sysreload-applied",
				"notification": map[string]string{"text": "The system has been reloaded with all outstanding changes"}})
			return
		}
```

```go
func TestPortalApplyChanges(t *testing.T) {
	ctx := context.Background()
	p := Profile{Host: "127.0.0.1", API: APIConfig{Enabled: true, BaseURL: fakePortal(t, true).URL}}
	msg, err := PortalApplyChanges(ctx, p, "good")
	if err != nil || !strings.Contains(msg, "reloaded") {
		t.Fatalf("want the portal's notification, got %q %v", msg, err)
	}
	if _, err := PortalApplyChanges(ctx, p, "bad"); err == nil {
		t.Fatal("without a session there must be no apply")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./pbx/... -run 'TestPinLists|TestApplyPins|TestPortalApplyChanges' -count=1`
Expected: FAIL, `undefined: PinLists` / `undefined: PortalApplyChanges`.

- [ ] **Step 3: Rename `describe` to `Describe`** in `pbx/check.go` (definition, its doc comment, the call in `Check`) and in `TestDescribe`. Doc comment: `// Describe turns channel and tool errors into short Spanish messages.`

- [ ] **Step 4: Append to `pbx/pins.go`** (add `"context"` to its imports):

```go
// PinList is a row of ombu_pin_lists with its current entry count.
type PinList struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Entries     int    `json:"entries"`
}

func PinLists(ctx context.Context, p Profile, sec Secrets) ([]PinList, error) {
	rows, err := MySQL(ctx, p, sec, "SELECT l.pin_list_id AS id, l.description AS description, COUNT(e.pin_list_entry_id) AS entries "+
		"FROM ombu_pin_lists l LEFT JOIN ombu_pin_list_entries e ON e.pin_list_id = l.pin_list_id "+
		"GROUP BY l.pin_list_id, l.description ORDER BY l.pin_list_id;")
	if err != nil {
		return nil, err
	}
	lists := make([]PinList, 0, len(rows))
	for _, r := range rows {
		id, _ := strconv.Atoi(r["id"])
		n, _ := strconv.Atoi(r["entries"])
		lists = append(lists, PinList{ID: id, Description: r["description"], Entries: n})
	}
	return lists, nil
}

// PinEntries returns PIN -> description for one list.
func PinEntries(ctx context.Context, p Profile, sec Secrets, listID int) (map[string]string, error) {
	rows, err := MySQL(ctx, p, sec, fmt.Sprintf("SELECT password AS pin, IFNULL(description, '') AS description FROM ombu_pin_list_entries WHERE pin_list_id = %d;", listID))
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r["pin"]] = r["description"]
	}
	return m, nil
}

// ApplyPins runs the add-only transaction, then stamps each line from a read-back of the list.
func ApplyPins(ctx context.Context, p Profile, sec Secrets, listID int, rows []PinRow) []PinRow {
	out := append([]PinRow(nil), rows...)
	_, err := MySQL(ctx, p, sec, InsertSQL(listID, rows))
	var after map[string]string
	verifyErr := error(nil)
	if err == nil {
		after, verifyErr = PinEntries(ctx, p, sec, listID)
	}
	for i := range out {
		switch out[i].Status {
		case PinExists:
			out[i].Status, out[i].Error = PinSkipped, "ya existía"
		case PinNoDesc:
			out[i].Status, out[i].Error = PinSkipped, "sin descripción"
		case PinNew:
			_, landed := after[out[i].PIN]
			switch {
			case err != nil:
				out[i].Status, out[i].Error = PinFailed, Describe(err)
			case verifyErr != nil:
				out[i].Status, out[i].Error = PinFailed, "no se pudo verificar: "+Describe(verifyErr)
			case !landed:
				out[i].Status, out[i].Error = PinFailed, "no se encontró después de aplicar"
			default:
				out[i].Status = PinApplied
			}
		}
	}
	return out
}
```

- [ ] **Step 5: Extract the portal session in `pbx/api.go` and add `PortalApplyChanges`.** Replace the file's body after the imports with (imports stay: `context`, `encoding/json`, `fmt`, `net/http`, `net/http/cookiejar`, `net/url`, `strings`):

```go
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
```

- [ ] **Step 6: Run tests**

Run: `go test ./pbx/... -count=1 -race && go vet ./pbx/...`
Expected: PASS (including the unchanged `TestPinnedTLS`, `TestAPICheckServerError`, `TestAPICheckLogin`), vet clean.

- [ ] **Step 7: Commit**

```bash
git add pbx/ && git commit -m "feat(pbx): read PIN lists/entries, apply with per-line read-back, portal apply-changes

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_0189kqTXMUpmvoaPH1rgHBFi"
```

---

### Task 6: `PinService` and wiring

**Files:**
- Create: `pins.go`, `pins_test.go` (package main)
- Modify: `profiles.go` (add `active`), `main.go` (register)

**Interfaces:**
- Consumes: everything from Tasks 2–5; `ProfileService.NextFolio` (existing)
- Produces (bound): `PinService.Lists() ([]pbx.PinList, error)`, `PickCSV() (string, error)`, `Preview(listID int, path string, includeEmpty bool) (PinPreview, error)`, `Apply(listID int) (PinApplyResult, error)`, `ApplyPortal() (string, error)`; models `PinPreview{ListID, Info pbx.CSVInfo, Rows []pbx.PinRow, New, Existing, NoDesc, Errors}` (json `listID, info, rows, new, existing, noDesc, errors`) and `PinApplyResult{Folio, Rows, Applied, Skipped, Failed}` (json `folio, rows, applied, skipped, failed`).

- [ ] **Step 1: Write the failing test** — create `pins_test.go`:

```go
package main

import (
	"strings"
	"testing"

	"github.com/skemono/Xorcom-Tools/pbx"
)

func TestPinPlanCheck(t *testing.T) {
	plan := &pinPlan{profileID: "A", listID: 7, rows: []pbx.PinRow{{PIN: "1", Status: pbx.PinNew}}}
	if err := plan.check("A", 7); err != nil {
		t.Fatalf("matching plan must pass: %v", err)
	}
	for name, err := range map[string]error{
		"no preview":   (*pinPlan)(nil).check("A", 7),
		"other PBX":    plan.check("B", 7),
		"other list":   plan.check("A", 8),
		"error rows":   (&pinPlan{profileID: "A", listID: 7, rows: []pbx.PinRow{{Status: pbx.PinNew}, {Status: pbx.PinError}}}).check("A", 7),
		"nothing new":  (&pinPlan{profileID: "A", listID: 7, rows: []pbx.PinRow{{Status: pbx.PinExists}}}).check("A", 7),
	} {
		if err == nil {
			t.Errorf("%s: Apply must be refused", name)
		}
	}
	if err := plan.check("B", 7); !strings.Contains(err.Error(), "no corresponde") {
		t.Errorf("message must say the preview belongs elsewhere: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test . -run TestPinPlanCheck -count=1` (needs `frontend/dist`, present from earlier builds)
Expected: FAIL, `undefined: pinPlan`.

- [ ] **Step 3: Add `active` to `profiles.go`**

```go
// active returns the active profile and its secrets for tools that act on the PBX over SSH.
func (s *ProfileService) active() (pbx.Profile, pbx.Secrets, error) {
	s.mu.Lock()
	p, ok := s.store.Get(s.store.ActiveID())
	s.mu.Unlock()
	if !ok {
		return pbx.Profile{}, pbx.Secrets{}, errors.New("no hay una PBX activa: elija una en el encabezado")
	}
	if !p.SSH.Enabled {
		return pbx.Profile{}, pbx.Secrets{}, errors.New("la PBX activa no tiene el canal SSH habilitado (F-01)")
	}
	sec, err := pbx.LoadSecrets(p.ID)
	if err != nil {
		return pbx.Profile{}, pbx.Secrets{}, fmt.Errorf("no se pudieron leer las contraseñas: %w", err)
	}
	return p, sec, nil
}
```

- [ ] **Step 4: Create `pins.go`**

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/skemono/Xorcom-Tools/pbx"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// PinService is F-02: add PINs from a CSV to an existing PIN list of the active PBX.
type PinService struct {
	profiles *ProfileService
	mu       sync.Mutex
	last     *pinPlan // exactly what Apply writes: the last preview
}

type pinPlan struct {
	profileID string
	listID    int
	rows      []pbx.PinRow
}

type PinPreview struct {
	ListID   int          `json:"listID"`
	Info     pbx.CSVInfo  `json:"info"`
	Rows     []pbx.PinRow `json:"rows"`
	New      int          `json:"new"`
	Existing int          `json:"existing"`
	NoDesc   int          `json:"noDesc"`
	Errors   int          `json:"errors"`
}

type PinApplyResult struct {
	Folio   int          `json:"folio"`
	Rows    []pbx.PinRow `json:"rows"`
	Applied int          `json:"applied"`
	Skipped int          `json:"skipped"`
	Failed  int          `json:"failed"`
}

const (
	pinTimeout   = 60 * time.Second
	applyTimeout = 5 * time.Minute // one NOT EXISTS lookup per row; the real list is 5 772 rows
	maxCSV       = 1 << 20
)

func (s *PinService) Lists() ([]pbx.PinList, error) {
	p, sec, err := s.profiles.active()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), pinTimeout)
	defer cancel()
	lists, err := pbx.PinLists(ctx, p, sec)
	if err != nil {
		return nil, errors.New(pbx.Describe(err))
	}
	return lists, nil
}

// PickCSV opens the native file dialog; "" means the user cancelled.
func (s *PinService) PickCSV() (string, error) {
	return application.Get().Dialog.OpenFile().
		SetTitle("Archivo CSV de PINes").
		AddFilter("CSV (*.csv)", "*.csv").
		PromptForSingleSelection()
}

func (s *PinService) Preview(listID int, path string, includeEmpty bool) (PinPreview, error) {
	p, sec, err := s.profiles.active()
	if err != nil {
		return PinPreview{}, err
	}
	st, err := os.Stat(path)
	if err != nil {
		return PinPreview{}, fmt.Errorf("no se pudo abrir el archivo: %v", err)
	}
	if st.Size() > maxCSV {
		return PinPreview{}, errors.New("el archivo pasa de 1 MB: no parece una lista de PINes")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return PinPreview{}, fmt.Errorf("no se pudo leer el archivo: %v", err)
	}
	info, rows, err := pbx.ParsePinCSV(raw, listID)
	if err != nil {
		return PinPreview{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), pinTimeout)
	defer cancel()
	existing, err := pbx.PinEntries(ctx, p, sec, listID)
	if err != nil {
		return PinPreview{}, errors.New(pbx.Describe(err))
	}
	rows = pbx.PlanPins(rows, existing, includeEmpty)
	s.mu.Lock()
	s.last = &pinPlan{profileID: p.ID, listID: listID, rows: rows}
	s.mu.Unlock()
	pv := PinPreview{ListID: listID, Info: info, Rows: rows}
	for _, r := range rows {
		switch r.Status {
		case pbx.PinNew:
			pv.New++
		case pbx.PinExists:
			pv.Existing++
		case pbx.PinNoDesc:
			pv.NoDesc++
		case pbx.PinError:
			pv.Errors++
		}
	}
	return pv, nil
}

func (s *PinService) Apply(listID int) (PinApplyResult, error) {
	p, sec, err := s.profiles.active()
	if err != nil {
		return PinApplyResult{}, err
	}
	s.mu.Lock()
	plan := s.last
	s.mu.Unlock()
	if err := plan.check(p.ID, listID); err != nil {
		return PinApplyResult{}, err
	}
	folio, err := s.profiles.NextFolio()
	if err != nil {
		return PinApplyResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), applyTimeout)
	defer cancel()
	res := PinApplyResult{Folio: folio, Rows: pbx.ApplyPins(ctx, p, sec, listID, plan.rows)}
	for _, r := range res.Rows {
		switch r.Status {
		case pbx.PinApplied:
			res.Applied++
		case pbx.PinSkipped:
			res.Skipped++
		case pbx.PinFailed:
			res.Failed++
		}
	}
	if res.Failed == 0 {
		s.mu.Lock()
		s.last = nil // done; a second Aplicar needs a fresh preview
		s.mu.Unlock()
	}
	return res, nil
}

// ApplyPortal runs the portal's own Apply with the profile's API credentials (F-01). It reloads
// the PBX with ALL outstanding portal changes; the UI only calls it after a two-step confirmation.
func (s *PinService) ApplyPortal() (string, error) {
	p, sec, err := s.profiles.active()
	if err != nil {
		return "", err
	}
	if !p.API.Enabled || sec.API == "" {
		return "", errors.New("falta la contraseña del portal: agréguela en F-01 (canal API)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), pinTimeout)
	defer cancel()
	msg, err := pbx.PortalApplyChanges(ctx, p, sec.API)
	if err != nil {
		return "", errors.New(pbx.Describe(err))
	}
	return msg, nil
}

// check refuses an Apply that would not write exactly what the user previewed.
func (pl *pinPlan) check(profileID string, listID int) error {
	if pl == nil {
		return errors.New("no hay vista previa: cargue el archivo primero")
	}
	if pl.profileID != profileID || pl.listID != listID {
		return errors.New("la vista previa no corresponde a la PBX y lista actuales: cárguela de nuevo")
	}
	n, bad := 0, 0
	for _, r := range pl.rows {
		switch r.Status {
		case pbx.PinNew:
			n++
		case pbx.PinError:
			bad++
		}
	}
	if bad > 0 {
		return fmt.Errorf("hay %d fila(s) con error: corrija el archivo y vuelva a cargarlo", bad)
	}
	if n == 0 {
		return errors.New("no hay PINes nuevos para aplicar")
	}
	return nil
}
```

- [ ] **Step 5: Register in `main.go`** — in the `Services` slice, after `application.NewService(profiles),`:

```go
			application.NewService(&PinService{profiles: profiles}),
```

- [ ] **Step 6: Test, vet, bindings**

```bash
go test . ./pbx/... -count=1 && go vet . ./pbx/... && wails3 generate bindings -clean=true -ts -i && grep -n "export function" frontend/bindings/github.com/skemono/Xorcom-Tools/pinservice.ts
```
Expected: PASS; `Apply`, `ApplyPortal`, `Lists`, `PickCSV`, `Preview` exported. Note whether `rows` is typed `PinRow[] | null` (frontend uses `?? []`).

- [ ] **Step 7: Commit**

```bash
git add pins.go pins_test.go profiles.go main.go frontend/bindings && git commit -m "feat: PinService (F-02) bound: lists, CSV preview, add-only apply

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_0189kqTXMUpmvoaPH1rgHBFi"
```

---

### Task 7: F-02 form in the Boleta world

**Files:**
- Create: `frontend/src/modules/Pines.svelte`
- Modify: `frontend/src/modules.ts`, `frontend/src/app.css`, `DESIGN.md`

**Interfaces:**
- Consumes: `PinService` bindings (T6), `app`, `errText` (state.svelte.ts), Task 1 rulings
- Produces: registry entry `F-02 PINes masivos`

- [ ] **Step 1: Re-read** `DESIGN.md` (Copy Color Rule, Reserved Marks Rule, stamp, ruled fields) and `impeccable/reference/craft-floor.md`. This is an extension inside the established world: no new tokens beyond the two classes below.

- [ ] **Step 2: Registry line** in `frontend/src/modules.ts`:

```ts
import Pines from './modules/Pines.svelte'
```
and after the F-01 entry:
```ts
  { code: 'F-02', id: 'pines', label: 'PINes masivos', component: Pines },
```

- [ ] **Step 3: Two shared classes** appended to `frontend/src/app.css` (per-line stamps and the struck state cell from the Reserved Marks Rule):

```css
/* Per-line sello for long tables; same ink and landing, small-label size. */
.stamp.mini { padding: 2px 6px 1px; font-size: 12px; letter-spacing: 0.1em; animation: none; } /* static: thousands of lines */
/* State cell: hollow pending, filled applied, struck failed. */
.mark { display: block; width: 12px; height: 12px; border: 2px solid var(--ink); }
.mark.filled { background: var(--ink); }
.mark.struck { border-color: var(--fail); background: linear-gradient(to top right, transparent calc(50% - 1px), var(--fail) calc(50% - 1px) calc(50% + 1px), transparent calc(50% + 1px)); }
```
Then delete the now-duplicate `.mark` / `.mark.filled` rules from `Conexiones.svelte`'s `<style>`.

- [ ] **Step 4: Create `frontend/src/modules/Pines.svelte`** (set the two constants from the Task 1 ledger rulings):

```svelte
<script lang="ts">
  import { untrack } from 'svelte'
  import { PinService } from '../../bindings/github.com/skemono/Xorcom-Tools'
  import type { PinApplyResult, PinPreview } from '../../bindings/github.com/skemono/Xorcom-Tools'
  import type { PinList, PinRow } from '../../bindings/github.com/skemono/Xorcom-Tools/pbx'
  import { app, errText } from '../state.svelte'

  // From the lab verification (plan Task 1); see the ledger rulings.
  const NEEDS_PORTAL_APPLY = false
  const RESAVE_LOSES_DESCRIPTIONS = false

  let lists = $state<PinList[]>([])
  let listID = $state(0)
  let listsError = $state('')
  let path = $state('')
  let preview = $state<PinPreview | null>(null)
  let result = $state<PinApplyResult | null>(null)
  let note = $state('')
  let includeEmpty = $state(false)
  let onlyIssues = $state(false)
  let confirmReload = $state(false)
  let portalMsg = $state('')

  const rows = $derived<PinRow[]>((result?.rows ?? preview?.rows) ?? [])
  // Thousands of lines: let the user see only the ones that need a look.
  const shown = $derived(onlyIssues ? rows.filter((r) => r.status !== 'nuevo' && r.status !== 'aplicado') : rows)
  const canApply = $derived(!!preview && !result && preview.errors === 0 && preview.new > 0 && !app.busy)
  const sepName = (s: string) => (s === 'tab' ? 'tabulador' : `«${s}»`)

  async function loadLists() {
    listsError = ''
    discard()
    if (!app.active) {
      lists = []
      return
    }
    app.busy = true
    try {
      lists = (await PinService.Lists()) ?? []
      if (!lists.some((l) => l.id === listID)) listID = lists[0]?.id ?? 0
    } catch (e) {
      lists = []
      listsError = errText(e)
    } finally {
      app.busy = false
    }
  }

  // A preview never survives a PBX change.
  let shownPBX: string | null = null
  $effect(() => {
    const id = app.active
    if (id !== shownPBX) {
      shownPBX = id
      untrack(() => loadLists())
    }
  })

  async function pick() {
    try {
      const p = await PinService.PickCSV()
      if (p) {
        path = p
        await runPreview()
      }
    } catch (e) {
      note = errText(e)
    }
  }

  async function runPreview() {
    if (!listID || !path.trim()) return
    app.busy = true
    note = ''
    result = null
    preview = null
    try {
      preview = await PinService.Preview(listID, path.trim(), includeEmpty)
      onlyIssues = preview.errors > 0
    } catch (e) {
      note = errText(e)
    } finally {
      app.busy = false
    }
  }

  async function apply() {
    if (!canApply) return
    app.busy = true
    note = ''
    try {
      result = await PinService.Apply(listID)
      app.folio = result.folio
      lists = (await PinService.Lists()) ?? lists // refreshed counts
    } catch (e) {
      note = errText(e)
    } finally {
      app.busy = false
    }
  }

  function discard() {
    preview = null
    result = null
    note = ''
    portalMsg = ''
    confirmReload = false
  }

  // The portal's Apply reloads the whole PBX with every pending portal change: two steps, like Eliminar.
  async function reloadPBX() {
    if (!confirmReload) {
      confirmReload = true
      setTimeout(() => (confirmReload = false), 4000)
      return
    }
    confirmReload = false
    app.busy = true
    try {
      portalMsg = await PinService.ApplyPortal()
    } catch (e) {
      portalMsg = errText(e)
    } finally {
      app.busy = false
    }
  }

  function planned(r: PinRow) {
    if (r.status === 'nuevo') return 'Nuevo'
    if (r.status === 'existe') return 'Ya existe: se omite'
    if (r.status === 'sindesc') return 'Sin descripción: se omite'
    return `Error: ${r.error}`
  }
</script>

<div class="page">
  <div class="title"><span class="code">F-02</span><h1>PINes masivos</h1></div>
  <p class="instr">Agregue PINes desde un CSV a una lista existente de la PBX activa; nada se escribe hasta Aplicar.</p>

  <h2 class="lbl section">Lista de PIN</h2>
  {#if listsError}
    <p class="notice">{listsError}</p>
  {:else if app.active && lists.length === 0 && !app.busy}
    <p class="notice">La PBX activa no tiene listas de PIN. Créela en el portal (con al menos un PIN) y presione Actualizar.</p>
  {/if}
  <div class="sheetrow">
    <label class="field grow">
      <span class="lbl">Lista</span>
      <select bind:value={listID} onchange={discard} disabled={app.busy || lists.length === 0}>
        {#each lists as l (l.id)}
          <option value={l.id}>Nº {l.id} · {l.description} ({l.entries} PIN{l.entries === 1 ? '' : 'es'})</option>
        {/each}
      </select>
    </label>
    <div class="field act"><button class="btn small" onclick={loadLists} disabled={app.busy || !app.active}>Actualizar</button></div>
  </div>
  {#if RESAVE_LOSES_DESCRIPTIONS}
    <p class="hint">No edite esta lista en el portal: al guardarla allí se pierden las descripciones.</p>
  {/if}

  <h2 class="lbl section">Archivo</h2>
  <div class="sheetrow">
    <label class="field grow">
      <span class="lbl">Archivo CSV</span>
      <input bind:value={path} onchange={runPreview} placeholder="C:\…\pines.csv" spellcheck="false" disabled={app.busy} />
    </label>
    <div class="field act">
      <button class="btn small" onclick={pick} disabled={app.busy || !listID}>Elegir…</button>
      <button class="btn small" onclick={runPreview} disabled={app.busy || !listID || !path.trim()}>Vista previa</button>
    </div>
  </div>
  <div class="format">
    <span class="lbl">Formato</span>
    <pre class="sample">PIN;Descripcion
4321;Dr. Jose Perez
5*55;Turno nocturno</pre>
    <p class="hint">
      Un PIN por fila: solo números y *. Separador ; o , · encabezado opcional · también acepta pin_list_id,PIN,descripción.
      Las ñ y tildes se reemplazan (José Peña → Jose Pena).
    </p>
  </div>

  {#if preview}
    <p class="detected">
      Separador {sepName(preview.info.separator)} · {preview.info.encoding} · {preview.info.header ? 'con encabezado' : 'sin encabezado'} · {preview.info.rows} filas
    </p>
    <div class="toggles">
      <label class="check"><input type="checkbox" bind:checked={includeEmpty} onchange={runPreview} disabled={app.busy || !!result} /> Incluir PINes sin descripción</label>
      <label class="check"><input type="checkbox" bind:checked={onlyIssues} /> Solo filas con observaciones</label>
      {#if onlyIssues}<span class="tag">Mostrando {shown.length} de {rows.length} filas</span>{/if}
    </div>
    <div class="copy" class:canary={!result}>
      <div class="strip">
        <span class="lbl">{result ? `Copia aplicada · Folio Nº ${String(result.folio).padStart(4, '0')}` : 'Copia — vista previa'}</span>
        <span class="sum">
          {#if result}
            {result.applied} aplicados · {result.skipped} omitidos · {result.failed} fallidos
          {:else}
            {preview.new} nuevos · {preview.existing} ya existen · {preview.noDesc} sin descripción · {preview.errors} con error
          {/if}
        </span>
      </div>
      <table class="lines">
        <thead>
          <tr>
            <th class="lbl num">Nº</th>
            <th class="cell-state"><span class="sr">Estado</span></th>
            <th class="lbl">PIN</th>
            <th class="lbl">Descripción</th>
            <th class="lbl">Resultado</th>
          </tr>
        </thead>
        <tbody>
          {#each shown as r (r.line)}
            <tr class:bad={r.status === 'error' || r.status === 'fallo'}>
              <td class="num">{String(r.line).padStart(3, '0')}</td>
              <td class="cell-state">
                <span class="mark" class:filled={r.status === 'aplicado'} class:struck={r.status === 'error' || r.status === 'fallo'}></span>
              </td>
              <td class="pin">{r.pin}</td>
              <td>{r.description}{#if r.filtered}<span class="tag"> (sin tildes)</span>{/if}</td>
              <td>
                {#if r.status === 'aplicado'}
                  <span class="stamp mini">Aplicado</span>
                {:else if r.status === 'fallo'}
                  <span class="stamp mini bad">Falló</span> <span class="why">{r.error}</span>
                {:else if r.status === 'omitido'}
                  <span class="quiet">Omitido</span> <span class="tag">{r.error}</span>
                {:else}
                  <span class:why={r.status === 'error'}>{planned(r)}</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    {#if result && NEEDS_PORTAL_APPLY && result.applied > 0}
      <p class="notice canary">
        Falta aplicar cambios en la PBX para que los PINes funcionen. Esto recarga la PBX y aplica también cualquier otro cambio pendiente del portal.
        <button class="btn small" onclick={reloadPBX} disabled={app.busy}>{confirmReload ? 'Confirmar: recargar la PBX' : 'Aplicar cambios en la PBX'}</button>
      </p>
      {#if portalMsg}<p class="detected">{portalMsg}</p>{/if}
    {/if}
  {/if}

  <div class="actions">
    <button class="btn primary" onclick={apply} disabled={!canApply}>
      {preview && !result ? `Aplicar ${preview.new} PIN${preview.new === 1 ? '' : 'es'}` : 'Aplicar'}
    </button>
    {#if preview}
      <button class="btn" onclick={discard} disabled={app.busy}>{result ? 'Nueva carga' : 'Descartar'}</button>
    {/if}
    <span class="note" role="status">
      {note || (preview && !result && preview.errors > 0 ? 'Corrija el archivo y vuelva a cargarlo: hay filas con error.' : '')}
    </span>
  </div>
</div>

<style>
  .page { padding: 14px 32px 0; }
  .title { display: flex; align-items: center; gap: 14px; }
  .title .code { padding: 5px 8px 4px; border: 2px solid var(--ink); font: 700 15px/1 var(--f-label); letter-spacing: 0.06em; color: var(--ink); }
  h1 { font: 700 30px/1 var(--f-label); letter-spacing: 0.03em; text-transform: uppercase; color: var(--ink); }
  .instr { max-width: 72ch; margin: 6px 0 12px; color: var(--ink-2); font-size: 14px; }
  .section { display: block; margin: 16px 0 6px; }
  .notice { margin-bottom: 8px; padding: 10px 14px; border: 1px solid var(--fail); background: var(--pink); }
  .notice.canary { border-color: var(--ink); background: var(--canary); }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }

  .sheetrow { display: flex; border-top: 2px solid var(--ink); border-left: 1px solid var(--ink); }
  .field { display: grid; gap: 4px; padding: 6px 12px 7px; border-right: 1px solid var(--ink); border-bottom: 1px solid var(--ink); min-width: 0; }
  .field.grow { flex: 1; }
  .field.act { display: flex; align-items: center; gap: 8px; }
  .field input, .field select { width: 100%; min-width: 0; padding: 0; border: 0; background: transparent; font: 500 16px/1.3 var(--f-data); color: var(--data); }
  .field input::placeholder { color: var(--ink-2); font-style: italic; font-weight: 400; }
  .hint { margin: 6px 0 0; color: var(--ink-2); font-size: 12px; line-height: 1.35; }

  .format { display: grid; grid-template-columns: auto 1fr; gap: 4px 16px; align-items: start; margin-top: 10px; }
  .format .lbl { grid-row: span 2; padding-top: 2px; }
  .sample { margin: 0; font: 400 12px/1.45 var(--f-mono); color: var(--data); }
  .format .hint { margin: 0; }
  .detected { margin: 14px 0 6px; color: var(--ink-2); font-size: 14px; }
  .toggles { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 20px; margin: 0 0 8px; }
  .check { display: inline-flex; align-items: center; gap: 6px; color: var(--ink); font-size: 13px; }
  .check input { width: 16px; height: 16px; margin: 0; accent-color: var(--ink); }

  .copy { border: 2px solid var(--ink); background: var(--paper); }
  .copy.canary { background: var(--canary); }
  .strip { display: flex; justify-content: space-between; align-items: baseline; padding: 8px 12px; border-bottom: 2px solid var(--ink); }
  .sum { font: 600 12px/1 var(--f-label); letter-spacing: 0.06em; text-transform: uppercase; color: var(--ink); }
  .lines { width: 100%; border-collapse: collapse; }
  .lines th { padding: 6px 10px 5px; border-bottom: 1px solid var(--ink); text-align: left; }
  .lines td { padding: 6px 10px; border-bottom: 1px solid rgb(35 61 122 / 0.22); vertical-align: middle; }
  .lines tr.bad td { background: var(--pink); }
  .num { width: 56px; color: var(--ink-2); font-weight: 500; }
  .cell-state { width: 34px; }
  .pin { font-weight: 600; letter-spacing: 0.02em; }
  .tag { color: var(--ink-2); font-size: 12px; }
  .why { color: var(--fail); font-size: 13px; }
  .quiet { font: 600 12px/1 var(--f-label); letter-spacing: 0.09em; text-transform: uppercase; color: var(--ink-2); }

  .actions {
    position: sticky; bottom: 0; z-index: 1;
    display: flex; flex-wrap: wrap; align-items: center; gap: 10px;
    margin-top: 20px; padding: 12px 0 16px;
    border-top: 2px solid var(--ink); background: var(--paper);
  }
  .note { color: var(--ink-2); font-size: 14px; }
</style>
```

- [ ] **Step 5: Type-check and build**

```bash
cd frontend && npx svelte-check --tsconfig ./tsconfig.json && cd .. && wails3 build && wails3 task build:server DEV=true
```
Expected: 0 errors; both binaries built.

- [ ] **Step 6: Scripted UI check against the lab (reads only)** — extend the scratchpad CDP tooling with `cdp-pines.mjs` (same helpers as `cdp-shots.mjs`): create a "Lab (temporal)" profile from env vars as in `cdp-lab.mjs` (SSH root password, API admin password, AMI off; trust the SSH fingerprint), open F-02 (click the `F-02` form tab), pick `PruebaIGSS` in the list select, set the path input to a scratchpad CSV and dispatch `change`, then check:
  1. `good.csv` (Windows-1252, `;`, header, "José Peña", `*99`, a PIN equal to the placeholder `<PH>`): detected line says `«;»`, `Windows-1252`, `con encabezado`; the row with <PH> says "Ya existe: se omite"; "José Peña" shows "Jose Pena (sin tildes)"; Aplicar is enabled and labelled with the new count.
  2. `bad.csv` (a `12a4` row and a repeated PIN): those rows are pink with their reasons; "Solo filas con observaciones" switched itself on; Aplicar is disabled; the note says to fix the file.
  2b. `good.csv` also has one row with an empty description: it reads "Sin descripción: se omite" and counts in "sin descripción"; ticking "Incluir PINes sin descripción" recomputes the preview and the row becomes "Nuevo" (Aplicar's count grows by one).
  3. Switching the header PBX picker is disabled while busy; changing the list select clears the preview.
  Capture `pines-preview.png` (good.csv), `pines-errors.png` (bad.csv) at 1240×700 and 1024×680 into `.impeccable/review/`. Do NOT press Aplicar here. Delete the temporary profile at the end and confirm `cmdkey /list` has no `UtilidadesXorcom` entries.

- [ ] **Step 7: One inspection round**: read the captures, fix in one batch (rebuild server, recapture once). Run `node "C:/Users/lordk/.claude/plugins/cache/impeccable/impeccable/4.1.1/skills/impeccable/scripts/detect.mjs" --json frontend/src` and fix mechanical findings. Spawn `impeccable:impeccable-finish-reviewer` with: the F-02 spec path, DESIGN.md path (established world, extension: judge against it, no direction contract change), artifact paths, the captures, detector output, and the craft-floor path; act on its disposition (two rounds max).

- [ ] **Step 8: DESIGN.md** — in the canary preview section, replace the "defined, NOT YET BUILT" heading/notes with the built pattern as shipped in F-02: canary copy behind the table, strip "Copia — vista previa" ↔ "Copia aplicada · Folio Nº", per-line `.stamp.mini`, `.mark` hollow/filled/struck, pink failed rows, sticky "Aplicar N PINes". Add `.stamp.mini` to the stamp component entry.

- [ ] **Step 9: Commit** (named paths only)

```bash
git add frontend/src/modules/Pines.svelte frontend/src/modules.ts frontend/src/app.css frontend/src/modules/Conexiones.svelte DESIGN.md && git commit -m "feat(ui): F-02 PINes masivos with canary preview and per-line stamps

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_0189kqTXMUpmvoaPH1rgHBFi"
```

---

### Task 8: Real load on the lab (with the user's go-ahead)

No new code. Proves the write path end to end.

- [ ] **Step 1: Ask the user**: "¿Aplico good.csv (N PINes de prueba) en PruebaIGSS del laboratorio?" Wait for yes.
- [ ] **Step 2:** Run the Task 7 Step 6 flow with good.csv, then press Aplicar. Expected: every new line stamps APLICADO, the placeholder line OMITIDO, folio issued, list count grows by N. Capture `pines-applied.png`.
- [ ] **Step 3: Independent read-back** with the probe: `SELECT password, description FROM ombu_pin_list_entries WHERE pin_list_id = <ID> ORDER BY pin_list_entry_id;` shows the N rows with filtered descriptions ("Jose Pena") and `NULL` where the CSV had none; no duplicates.
- [ ] **Step 4: Re-apply guard**: load the same good.csv again: every former new line now says "Ya existe: se omite"; Aplicar disabled ("no hay PINes nuevos").
- [ ] **Step 5: If `NEEDS_PORTAL_APPLY`**: confirm the notice shows after Aplicar. Ask the user: "¿Presiono Aplicar cambios en la PBX? Recarga el laboratorio y aplica cualquier cambio pendiente del portal." On yes, click it twice (two-step) and check the portal's text "The system has been reloaded…" appears; then re-run the Task 1 greps to confirm the new PINs reached AstDB/conf; optionally the user tests one PIN in a call.
- [ ] **Step 5b: Real file, preview only (no write)**: point F-02 at `docs/test_data/PinList.csv` on `PruebaIGSS`. Expected, from the aggregate check: 5 772 rows, `«,»` · UTF-8 · con encabezado, 8 error rows (duplicate pairs at lines 12/13, 730/731, 3627/4434, 3601/5120), 1 516+26 lines marked "(sin tildes)", 0 sin descripción, Aplicar blocked, "Solo filas con observaciones" on. Note how long the preview takes. Capture nothing from this file (personal data): report counts only. A full real load happens only after the user fixes the duplicates and asks for it.
- [ ] **Step 6: Cleanup (ask the user first)**: delete the test rows by PIN from `PruebaIGSS` with the probe; ask whether to keep or delete the `PruebaIGSS` list itself (portal). Delete the temporary app profile; `cmdkey /list` clean; remove `%APPDATA%\UtilidadesXorcom\profiles.json` if it holds only test data.
- [ ] **Step 7:** Ledger the results (no commit needed unless a fix was required, which then goes through TDD in the owning task's files).
