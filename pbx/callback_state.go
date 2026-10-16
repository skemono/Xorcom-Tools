package pbx

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CallbackState is everything F-03 needs to know about the active PBX, read in one SSH session.
type CallbackState struct {
	HasCC        bool                `json:"hasCC"`
	Queues       []CallbackQueue     `json:"queues"`
	Apps         []CallbackApp       `json:"apps"`
	IVRs         []CallbackIVR       `json:"ivrs"`
	Priorities   []CallbackPriority  `json:"priorities"`
	Recordings   []CallbackRecording `json:"recordings"`
	RecordingDir string              `json:"recordingDir"`
	FileExists   bool                `json:"fileExists"`
	File         string              `json:"file"`
	LogOwner     string              `json:"logOwner"` // "" = no log file
	Contexts     int                 `json:"contexts"` // how many of the 3 contexts Asterisk has loaded
	TakenCodes   map[string]bool     `json:"takenCodes"`
	Waiting      []CallbackWaiting   `json:"waiting"`
}

type CallbackQueue struct {
	ID          int    `json:"id"`
	Extension   string `json:"extension"`
	Description string `json:"description"`
	IVRID       int    `json:"ivrID"`
	IVRName     string `json:"ivrName"`
	MemberGosub string `json:"memberGosub"`
}

// CallbackApp is a custom app with its destination resolved: DestKind is the destination category name
// (custom, queue_priority, …), DestIndex its row id, Custom the "context,exten,priority" of a Custom one.
type CallbackApp struct {
	ID          int    `json:"id"`
	Extension   string `json:"extension"`
	Description string `json:"description"`
	DestKind    string `json:"destKind"`
	DestIndex   int    `json:"destIndex"`
	Custom      string `json:"custom"`
}

type CallbackIVR struct {
	ID          int                `json:"id"`
	Description string             `json:"description"`
	Entries     []CallbackIVREntry `json:"entries"`
}

type CallbackIVREntry struct {
	Option    string `json:"option"`
	Enabled   bool   `json:"enabled"`
	DestKind  string `json:"destKind"`
	DestIndex int    `json:"destIndex"`
}

type CallbackPriority struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	DestKind    string `json:"destKind"`
	DestIndex   int    `json:"destIndex"`
}

type CallbackRecording struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// CallbackWaiting is one callback request still in AstDB: the queue, its key (the caller's uniqueid) and the number.
type CallbackWaiting struct {
	Queue  string `json:"queue"`
	ID     string `json:"id"`
	Number string `json:"number"`
}

const (
	callbackFile = "/etc/asterisk/ombutel/extensions__65-callback.conf"
	callbackLog  = "/var/log/cpbx/callback.log"
	sectionMark  = "@@f03 "
)

// destJoin resolves an ombu_destinations id column to (dest_kind, dest_index).
func destJoin(col string) string {
	return " LEFT JOIN ombu_destinations d ON d.destination_id = " + col +
		" LEFT JOIN ombu_destination_categories c ON c.category_id = d.category_id"
}

const destCols = "IFNULL(c.name, '') AS dest_kind, IFNULL(d.`index`, 0) AS dest_index"

var callbackSQL = []struct{ name, sql string }{
	{"modules", "SELECT name FROM ombu_modules WHERE name = 'supervision';"},
	{"queues", "SELECT q.queue_id AS id, q.extension AS extension, IFNULL(q.description, '') AS description, IFNULL(q.ivr_id, 0) AS ivr_id, " +
		"IFNULL(i.description, '') AS ivr_name, IFNULL(p.value, '') AS membergosub FROM ombu_queues q " +
		"LEFT JOIN ombu_ivrs i ON i.ivr_id = q.ivr_id LEFT JOIN ombu_queue_params p ON p.queue_id = q.queue_id AND p.name = 'membergosub' ORDER BY q.extension;"},
	// Task 1 confirms: a Custom destination's index points at ombu_custom_contexts.
	{"apps", "SELECT a.custom_application_id AS id, a.extension AS extension, IFNULL(a.description, '') AS description, " + destCols + ", " +
		"IFNULL((SELECT CONCAT(x.context, ',', x.extension, ',', x.priority) FROM ombu_custom_contexts x WHERE c.name = 'custom' AND x.custom_context_id = d.`index`), '') AS custom " +
		"FROM ombu_custom_applications a" + destJoin("a.destination_id") + " ORDER BY a.extension;"},
	{"ivrs", "SELECT i.ivr_id AS id, IFNULL(i.description, '') AS description, IFNULL(e.`option`, '') AS `option`, IFNULL(e.enabled, 'no') AS enabled, " + destCols +
		" FROM ombu_ivrs i LEFT JOIN ombu_ivr_entries e ON e.ivr_id = i.ivr_id" + destJoin("e.destination_id") + " ORDER BY i.ivr_id DESC, e.sort, e.ivr_entry_id;"},
	{"priorities", "SELECT p.queue_priority_id AS id, IFNULL(p.description, '') AS description, p.priority AS priority, " + destCols +
		" FROM ombu_queue_priorities p" + destJoin("p.destination_id") + " ORDER BY p.queue_priority_id;"},
	{"recordings", "SELECT recording_id AS id, name FROM ombu_recordings WHERE original_filename <> '' ORDER BY name;"},
}

// callbackStateScript reads the whole state; every section starts with "@@f03 <name>". codes must be digits.
func callbackStateScript(codes []string) string {
	var b strings.Builder
	for _, q := range callbackSQL {
		fmt.Fprintf(&b, "echo '%s%s'\n%s <<'SQL' || echo '%serror %s'\n%s\nSQL\n", sectionMark, q.name, mysqlCmd, sectionMark, q.name, q.sql)
	}
	fmt.Fprintf(&b, "echo '%srecdir'\nls -d /var/lib/ombutel/static/*/recordings 2>/dev/null\n", sectionMark)
	fmt.Fprintf(&b, "echo '%sfile'\nif [ -e %s ]; then echo 1; base64 -w0 %s; echo; fi\n", sectionMark, callbackFile, callbackFile)
	fmt.Fprintf(&b, "echo '%slog'\nstat -c %%U %s 2>/dev/null\n", sectionMark, callbackLog)
	fmt.Fprintf(&b, "echo '%scontexts'\nfor c in create_callback check_position callback_out; do asterisk -rx \"dialplan show $c\" | grep -q \"Context '$c'\" && echo $c; done\n", sectionMark)
	fmt.Fprintf(&b, "echo '%staken'\nfor c in %s; do asterisk -rx \"dialplan show $c@cos-all-local\" | grep -q 'There is no existence' || echo $c; done\n", sectionMark, strings.Join(codes, " "))
	fmt.Fprintf(&b, "echo '%swaiting'\ni=$(asterisk -rx 'database get pbx instance' | sed -n 's/^Value: //p')\nif [ -n \"$i\" ]; then asterisk -rx \"database show $i/callback\"; fi\n", sectionMark)
	return b.String()
}

// ReadCallbackState runs the state script over SSH. codes are the dial codes to check (CandidateCodes).
func ReadCallbackState(ctx context.Context, p Profile, sec Secrets, codes []string) (CallbackState, error) {
	for _, c := range codes {
		if !reDigits.MatchString(c) {
			return CallbackState{}, fmt.Errorf("código inválido «%s»", c)
		}
	}
	if len(codes) == 0 {
		codes = []string{"0"} // keeps the for loop valid; "0" is never a free app code anyway
	}
	out, err := SSHExec(ctx, p, sec.SSH, sec.SSHKey, "bash -s", strings.NewReader(callbackStateScript(codes)))
	if err != nil {
		return CallbackState{}, fmt.Errorf("no se pudo leer el estado de la PBX: %w", err)
	}
	return parseCallbackState(out)
}

var reWaiting = regexp.MustCompile(`^/[^/]+/callback/([0-9]+)/([0-9]+\.[0-9]+)\s*:\s*([0-9]*)\s*$`)

func parseCallbackState(out string) (CallbackState, error) {
	sec := map[string]string{}
	name := ""
	for _, line := range strings.SplitAfter(out, "\n") {
		if n, ok := strings.CutPrefix(line, sectionMark); ok {
			name = strings.TrimSpace(n)
			sec[name] = ""
			continue
		}
		if name != "" {
			sec[name] += line
		}
	}
	for k := range sec {
		if x, ok := strings.CutPrefix(k, "error "); ok {
			return CallbackState{}, fmt.Errorf("la PBX no respondió a la consulta «%s»", x)
		}
	}
	if _, ok := sec["waiting"]; !ok {
		return CallbackState{}, errors.New("respuesta inesperada de la PBX al leer el estado (¿es una CompletePBX 5?)")
	}
	atoi := func(s string) int { n, _ := strconv.Atoi(s); return n }
	lines := func(s string) []string { return strings.Fields(s) }
	st := CallbackState{TakenCodes: map[string]bool{}}
	for _, r := range parseBatch(sec["modules"]) {
		st.HasCC = st.HasCC || r["name"] == "supervision"
	}
	for _, r := range parseBatch(sec["queues"]) {
		st.Queues = append(st.Queues, CallbackQueue{ID: atoi(r["id"]), Extension: r["extension"], Description: r["description"],
			IVRID: atoi(r["ivr_id"]), IVRName: r["ivr_name"], MemberGosub: r["membergosub"]})
	}
	for _, r := range parseBatch(sec["apps"]) {
		st.Apps = append(st.Apps, CallbackApp{ID: atoi(r["id"]), Extension: r["extension"], Description: r["description"],
			DestKind: r["dest_kind"], DestIndex: atoi(r["dest_index"]), Custom: r["custom"]})
	}
	for _, r := range parseBatch(sec["ivrs"]) {
		id := atoi(r["id"])
		if n := len(st.IVRs); n == 0 || st.IVRs[n-1].ID != id {
			st.IVRs = append(st.IVRs, CallbackIVR{ID: id, Description: r["description"]})
		}
		if r["option"] != "" || r["dest_kind"] != "" {
			iv := &st.IVRs[len(st.IVRs)-1]
			iv.Entries = append(iv.Entries, CallbackIVREntry{Option: r["option"], Enabled: r["enabled"] == "yes", DestKind: r["dest_kind"], DestIndex: atoi(r["dest_index"])})
		}
	}
	for _, r := range parseBatch(sec["priorities"]) {
		st.Priorities = append(st.Priorities, CallbackPriority{ID: atoi(r["id"]), Description: r["description"], Priority: atoi(r["priority"]),
			DestKind: r["dest_kind"], DestIndex: atoi(r["dest_index"])})
	}
	for _, r := range parseBatch(sec["recordings"]) {
		st.Recordings = append(st.Recordings, CallbackRecording{ID: atoi(r["id"]), Name: r["name"]})
	}
	if d := lines(sec["recdir"]); len(d) == 1 { // exactly one portal instance dir, or no playback
		st.RecordingDir = d[0]
	}
	if f := lines(sec["file"]); len(f) >= 1 && f[0] == "1" {
		st.FileExists = true
		if len(f) == 2 {
			raw, err := base64.StdEncoding.DecodeString(f[1])
			if err != nil {
				return CallbackState{}, fmt.Errorf("no se pudo leer %s: %v", callbackFile, err)
			}
			st.File = string(raw)
		}
	}
	if o := lines(sec["log"]); len(o) == 1 {
		st.LogOwner = o[0]
	}
	st.Contexts = len(lines(sec["contexts"]))
	for _, c := range lines(sec["taken"]) {
		st.TakenCodes[c] = true
	}
	for _, line := range strings.Split(sec["waiting"], "\n") {
		if m := reWaiting.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			st.Waiting = append(st.Waiting, CallbackWaiting{Queue: m[1], ID: m[2], Number: m[3]})
		}
	}
	return st, nil
}
