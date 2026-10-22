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
	OtherFiles   []string            `json:"otherFiles"` // other dialplan files that define them (a hand-made install)
	TakenCodes   map[string]bool     `json:"takenCodes"`
	Waiting      []CallbackWaiting   `json:"waiting"`
	Categories   map[string]string   `json:"categories"` // destination category name -> id
}

// CallbackQueue is one queue as the portal stores it, plus what Asterisk runs for it now: LiveContext and
// LiveGosub come from the rendered queues file, which changes only on the portal's Apply ("" = not there).
type CallbackQueue struct {
	ID          int    `json:"id"`
	Extension   string `json:"extension"`
	Description string `json:"description"`
	IVRID       int    `json:"ivrID"`
	IVRName     string `json:"ivrName"`
	MemberGosub string `json:"memberGosub"`
	LiveContext string `json:"liveContext"`
	LiveGosub   string `json:"liveGosub"`
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
	queuesFile   = "/etc/asterisk/ombutel/queues__50-1-main.conf"
	sectionMark  = "@@f03 "
)

// renderedAwk prints "<ext>\t<context>\t<membergosub>" for every [Q<ext>] block of the rendered queues
// file; it takes both "key=value" and "key = value".
const renderedAwk = `function flush() { if (q != "") print q "\t" c "\t" g }
{ sub(/\r$/, "") }
/^\[/ { flush(); q = ""; c = ""; g = ""; e = index($0, "]"); if (substr($0, 2, 1) == "Q" && e > 3) q = substr($0, 3, e - 3); next }
q != "" { n = index($0, "="); if (n == 0) next; k = substr($0, 1, n - 1); v = substr($0, n + 1)
  gsub(/^[ \t]+|[ \t]+$/, "", k); gsub(/^[ \t]+|[ \t]+$/, "", v)
  if (k == "context") c = v; else if (k == "membergosub") g = v }
END { flush() }`

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
	{"apps", "SELECT a.custom_application_id AS id, a.extension AS extension, IFNULL(a.description, '') AS description, " + destCols + ", " +
		"IFNULL((SELECT CONCAT(x.context, ',', x.extension, ',', x.priority) FROM ombu_custom_contexts x WHERE c.name = 'custom' AND x.custom_context_id = d.`index`), '') AS custom " +
		"FROM ombu_custom_applications a" + destJoin("a.destination_id") + " ORDER BY a.extension;"},
	{"ivrs", "SELECT i.ivr_id AS id, IFNULL(i.description, '') AS description, IFNULL(e.`option`, '') AS `option`, IFNULL(e.enabled, 'no') AS enabled, " + destCols +
		" FROM ombu_ivrs i LEFT JOIN ombu_ivr_entries e ON e.ivr_id = i.ivr_id" + destJoin("e.destination_id") + " ORDER BY i.ivr_id DESC, e.sort, e.ivr_entry_id;"},
	{"priorities", "SELECT p.queue_priority_id AS id, IFNULL(p.description, '') AS description, p.priority AS priority, " + destCols +
		" FROM ombu_queue_priorities p" + destJoin("p.destination_id") + " ORDER BY p.queue_priority_id;"},
	{"recordings", "SELECT recording_id AS id, name FROM ombu_recordings WHERE original_filename <> '' ORDER BY name;"},
	{"categories", "SELECT category_id AS id, name FROM ombu_destination_categories WHERE name IN ('custom','custom_application','queue','queue_priority','terminate_call');"},
}

// sqlSection adds one query to a bash -s script: its "@@f03 <name>" mark, then its rows (or an error mark).
func sqlSection(b *strings.Builder, name, sql string) {
	fmt.Fprintf(b, "echo '%s%s'\n%s <<'SQL' || echo '%serror %s'\n%s\nSQL\n", sectionMark, name, mysqlCmd, sectionMark, name, sql)
}

// callbackStateScript reads the whole state; every section starts with "@@f03 <name>". codes must be digits.
func callbackStateScript(codes []string) string {
	var b strings.Builder
	for _, q := range callbackSQL {
		sqlSection(&b, q.name, q.sql)
	}
	fmt.Fprintf(&b, "echo '%srendered'\nif [ -e %s ]; then awk '%s' %s || echo '%serror rendered'; fi\n", sectionMark, queuesFile, renderedAwk, queuesFile, sectionMark)
	fmt.Fprintf(&b, "echo '%srecdir'\nls -d /var/lib/ombutel/static/*/recordings 2>/dev/null\n", sectionMark)
	fmt.Fprintf(&b, "echo '%sfile'\nif [ -e %s ]; then echo 1; base64 -w0 %s; echo; fi\n", sectionMark, callbackFile, callbackFile)
	fmt.Fprintf(&b, "echo '%slog'\nstat -c %%U %s 2>/dev/null\n", sectionMark, callbackLog)
	fmt.Fprintf(&b, "echo '%scontexts'\nfor c in create_callback check_position callback_out; do asterisk -rx \"dialplan show $c\" | grep -q \"Context '$c'\" && echo $c; done\n", sectionMark)
	fmt.Fprintf(&b, "echo '%sothers'\ngrep -l -E '^\\[(create_callback|check_position|callback_out)\\]' /etc/asterisk/*.conf /etc/asterisk/ombutel/*.conf 2>/dev/null | grep -v -x -F %s; true\n", sectionMark, callbackFile)
	fmt.Fprintf(&b, "echo '%staken'\nfor c in %s; do asterisk -rx \"dialplan show $c@cos-all-local\" | grep -q 'There is no existence' || echo $c; done\n", sectionMark, strings.Join(codes, " "))
	fmt.Fprintf(&b, "echo '%swaiting'\nasterisk -rx 'database show f03callback'\n", sectionMark)
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

var reWaiting = regexp.MustCompile(`^/f03callback/([0-9]+)/([0-9]+\.[0-9]+)\s*:\s*([0-9]*)\s*$`)

// splitSections cuts a script's output at its "@@f03 <name>" marks; an "error <name>" mark is an error.
func splitSections(out string) (map[string]string, error) {
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
			return nil, fmt.Errorf("la PBX no respondió a la consulta «%s»", x)
		}
	}
	return sec, nil
}

func parseCallbackState(out string) (CallbackState, error) {
	sec, err := splitSections(out)
	if err != nil {
		return CallbackState{}, err
	}
	if _, ok := sec["waiting"]; !ok {
		return CallbackState{}, errors.New("respuesta inesperada de la PBX al leer el estado (¿es una CompletePBX 5?)")
	}
	atoi := func(s string) int { n, _ := strconv.Atoi(s); return n }
	lines := func(s string) []string { return strings.Fields(s) }
	st := CallbackState{TakenCodes: map[string]bool{}, Categories: map[string]string{}}
	for _, r := range parseBatch(sec["modules"]) {
		st.HasCC = st.HasCC || r["name"] == "supervision"
	}
	rendered := map[string][]string{}
	for _, line := range strings.Split(sec["rendered"], "\n") {
		if f := strings.Split(strings.TrimRight(line, "\r"), "\t"); len(f) == 3 {
			rendered[f[0]] = f[1:]
		}
	}
	for _, r := range parseBatch(sec["queues"]) {
		q := CallbackQueue{ID: atoi(r["id"]), Extension: r["extension"], Description: r["description"],
			IVRID: atoi(r["ivr_id"]), IVRName: r["ivr_name"], MemberGosub: r["membergosub"]}
		if f, ok := rendered[q.Extension]; ok {
			q.LiveContext, q.LiveGosub = f[0], f[1]
		}
		st.Queues = append(st.Queues, q)
	}
	for _, r := range parseBatch(sec["categories"]) {
		st.Categories[r["name"]] = r["id"]
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
	st.OtherFiles = lines(sec["others"])
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
