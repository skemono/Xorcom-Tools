package pbx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// ombu_destination_categories ids recorded on CompletePBX 5: what the portal's own forms post as mod_dest
// (the state reads resolve the same categories by name).
const (
	catCustom      = "25"
	catCustomApp   = "4"
	catQueue       = "10"
	catQueuePrio   = "11"
	catTerminate   = "24" // with index 1 = Hangup: the required return/invalid/timeout destination
	callbackDesc   = "Callback"
	gosubNameField = "advanced_name[]"
	gosubValField  = "advanced_value[]"
)

// knownCategories: the names the state script reads from ombu_destination_categories and the ids the forms post.
var knownCategories = map[string]string{"custom": catCustom, "custom_application": catCustomApp, "queue": catQueue,
	"queue_priority": catQueuePrio, "terminate_call": catTerminate}

// CallbackPortal is one logged-in portal session for a whole Aplicar/Quitar.
type CallbackPortal struct {
	pt  *portal
	p   Profile
	sec Secrets
}

func OpenCallbackPortal(ctx context.Context, p Profile, sec Secrets) (*CallbackPortal, error) {
	if !p.API.Enabled || sec.API == "" {
		return nil, errors.New("falta la contraseña del portal: agréguela en F-01 (canal API)")
	}
	pt, err := newPortal(p)
	if err != nil {
		return nil, err
	}
	if err := pt.login(ctx, portalUser(p), sec.API); err != nil {
		return nil, err
	}
	return &CallbackPortal{pt: pt, p: p, sec: sec}, nil
}

func (cp *CallbackPortal) Close() { cp.pt.client.CloseIdleConnections() }

// queueTablesSQL finds every other table with a queue_id column (agents, departments, …) and its
// primary key columns.
const queueTablesSQL = "SELECT q.table_name AS t, IFNULL(GROUP_CONCAT(k.column_name), '') AS pk FROM information_schema.columns q " +
	"LEFT JOIN information_schema.columns k ON k.table_schema = q.table_schema AND k.table_name = q.table_name AND k.column_key = 'PRI' " +
	"WHERE q.table_schema = 'ombutel' AND q.column_name = 'queue_id' AND q.table_name NOT IN ('ombu_queues', 'ombu_queue_params') " +
	"GROUP BY q.table_name ORDER BY q.table_name;"

var reTableName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// queueSnapshot is everything the portal's queue form can save, compared around a save: the queue's
// row, its CUSTOM params ("param:<name>") and its rows in every other table with a queue_id column
// ("table:<name>#<n>", sorted). A single-column primary key named *_id is left out: the portal may
// renumber those rows on a save without changing them.
func queueSnapshot(ctx context.Context, p Profile, sec Secrets, id int) (map[string]string, error) {
	tables, err := MySQL(ctx, p, sec, queueTablesSQL)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	sqlSection(&b, "queue", fmt.Sprintf("SELECT * FROM ombu_queues WHERE queue_id = %d;", id))
	sqlSection(&b, "params", fmt.Sprintf("SELECT name, value FROM ombu_queue_params WHERE queue_id = %d;", id))
	for _, t := range tables {
		if !reTableName.MatchString(t["t"]) {
			return nil, fmt.Errorf("la PBX tiene una tabla de colas con un nombre inesperado «%s»", t["t"])
		}
		sqlSection(&b, "table "+t["t"], fmt.Sprintf("SELECT * FROM `%s` WHERE queue_id = %d;", t["t"], id))
	}
	out, err := SSHExec(ctx, p, sec.SSH, sec.SSHKey, "bash -s", strings.NewReader(b.String()))
	if err != nil {
		return nil, fmt.Errorf("MySQL: %w", err)
	}
	secs, err := splitSections(out)
	if err != nil {
		return nil, err
	}
	need := []string{"queue", "params"}
	for _, t := range tables {
		need = append(need, "table "+t["t"])
	}
	for _, n := range need { // a cut-short answer must not look like "nothing there"
		if _, ok := secs[n]; !ok {
			return nil, fmt.Errorf("respuesta incompleta de la PBX al leer la cola %d", id)
		}
	}
	rows := parseBatch(secs["queue"])
	if len(rows) != 1 {
		return nil, fmt.Errorf("la cola %d no existe", id)
	}
	snap := rows[0]
	for _, r := range parseBatch(secs["params"]) {
		snap["param:"+r["name"]] = r["value"]
	}
	for _, t := range tables {
		skip := t["pk"]
		if strings.Contains(skip, ",") || !strings.HasSuffix(skip, "_id") || skip == "queue_id" {
			skip = "" // a composite key or a non-id key is data
		}
		var canon []string
		for _, r := range parseBatch(secs["table "+t["t"]]) {
			delete(r, skip)
			cols := make([]string, 0, len(r))
			for k, v := range r {
				cols = append(cols, k+"="+strconv.Quote(v))
			}
			sort.Strings(cols)
			canon = append(canon, strings.Join(cols, " "))
		}
		sort.Strings(canon)
		for i, c := range canon {
			snap[fmt.Sprintf("table:%s#%d", t["t"], i)] = c
		}
	}
	return snap, nil
}

// queueChanges lists every difference between two snapshots except the two fields F-03 may change.
func queueChanges(before, after map[string]string) []string {
	keys := map[string]bool{}
	for k := range before {
		keys[k] = true
	}
	for k := range after {
		keys[k] = true
	}
	var out []string
	for k := range keys {
		if k != "ivr_id" && k != "param:membergosub" && before[k] != after[k] {
			out = append(out, fmt.Sprintf("%s: «%s» → «%s»", k, before[k], after[k]))
		}
	}
	sort.Strings(out)
	return out
}

// guardQueue runs save on queue q and fails if it changed anything but IVR and membergosub.
// The portal has already saved by then, so the message says exactly what to undo by hand.
func (cp *CallbackPortal) guardQueue(ctx context.Context, q CallbackQueue, save func() error) error {
	before, err := queueSnapshot(ctx, cp.p, cp.sec, q.ID)
	if err != nil {
		return err
	}
	saveErr := save()
	after, err := queueSnapshot(ctx, cp.p, cp.sec, q.ID)
	if err != nil {
		if saveErr != nil {
			return fmt.Errorf("%w; además, no se pudo verificar la cola %s después de guardar: %v", saveErr, q.Extension, err)
		}
		return fmt.Errorf("no se pudo verificar la cola %s después de guardar: %w", q.Extension, err)
	}
	if d := queueChanges(before, after); len(d) > 0 {
		msg := fmt.Sprintf("el portal cambió otros campos de la cola %s (revíselos en el portal): %s", q.Extension, strings.Join(d, "; "))
		if saveErr != nil {
			return fmt.Errorf("%w; además, %s", saveErr, msg)
		}
		return errors.New(msg)
	}
	return saveErr
}

// CallbackStepRunner performs one plan step: SSH steps directly, portal steps through cp. Portal steps
// re-read the state first for the ids they need (the app id for the IVR entry, the priority id for its app).
func CallbackStepRunner(p Profile, sec Secrets, cp *CallbackPortal, plan CallbackPlan) func(context.Context, CallbackStep) error {
	return func(ctx context.Context, s CallbackStep) error {
		switch s.Kind {
		case StepFile:
			return InstallCallbackFile(ctx, p, sec, plan.Dialplan)
		case StepDelFile:
			return RemoveCallbackFile(ctx, p, sec)
		case StepLog:
			return EnsureCallbackLog(ctx, p, sec)
		case StepDropWaiting:
			return DropCallbacks(ctx, p, sec, s.Queue)
		}
		st, err := ReadCallbackState(ctx, p, sec, nil)
		if err != nil {
			return err
		}
		var q *CallbackQueue
		if s.Queue != "" {
			if q = st.Queue(s.Queue); q == nil {
				return fmt.Errorf("la cola %s ya no existe", s.Queue)
			}
		}
		switch s.Kind {
		case StepPriority, StepPrioApp, StepQueue, StepDetach, StepDelPrioApp, StepDelPriority:
			if q == nil {
				return fmt.Errorf("el paso «%s» necesita una cola", s.Kind)
			}
		}
		switch s.Kind {
		case StepApp:
			return cp.createCallbackApp(ctx, s.Code)
		case StepIVR:
			app := st.CallbackApp()
			if app == nil {
				return errors.New("falta la aplicación de devolución de llamada")
			}
			return cp.createCallbackIVR(ctx, plan.Config.Key, *app)
		case StepPriority:
			return cp.createPriority(ctx, *q, plan.Config.Priority)
		case StepPrioApp:
			pr := st.Priority(*q)
			if pr == nil {
				return fmt.Errorf("falta la prioridad de la cola %s", s.Queue)
			}
			return cp.createPrioApp(ctx, s.Code, *q, *pr)
		case StepQueue:
			iv := st.CallbackIVR()
			if iv == nil {
				return errors.New("falta el IVR de devolución de llamada")
			}
			if why := st.foreign(*q); why != "" { // someone set an IVR or membergosub since the preview
				return errors.New(why)
			}
			return cp.guardQueue(ctx, *q, func() error { return cp.setQueue(ctx, *q, iv.ID, memberGosub) })
		case StepDetach:
			keep := detachIVR(st, *q)
			return cp.guardQueue(ctx, *q, func() error { return cp.setQueue(ctx, *q, keep, "") })
		case StepDelPrioApp:
			if pa := st.PrioApp(st.Priority(*q)); pa != nil {
				return cp.deleteApp(ctx, pa.ID)
			}
			return nil
		case StepDelPriority:
			if pr := st.Priority(*q); pr != nil {
				return cp.deletePriority(ctx, pr.ID)
			}
			return nil
		case StepDelIVR:
			if iv := st.CallbackIVR(); iv != nil {
				return cp.deleteIVR(ctx, iv.ID)
			}
			return nil
		case StepDelApp:
			if app := st.CallbackApp(); app != nil {
				return cp.deleteApp(ctx, app.ID)
			}
			return nil
		}
		return fmt.Errorf("paso desconocido «%s»", s.Kind)
	}
}

// detachIVR is the IVR id a detach leaves on q: a foreign IVR stays (only membergosub goes), F-03's own or none -> 0.
func detachIVR(st CallbackState, q CallbackQueue) int {
	if iv := st.CallbackIVR(); q.IVRID != 0 && (iv == nil || q.IVRID != iv.ID) {
		return q.IVRID
	}
	return 0
}

// customAppFields is the custom_app save: a destination plus the required return destination (Hangup).
func customAppFields(code, desc, modDest, destIndex, custom string) [][2]string {
	return [][2]string{{"extension", code}, {"description", desc}, {"status", "yes"}, {"direct", "yes"},
		{"mod_dest", modDest}, {"destination_index", destIndex}, {"custom_destination", custom},
		{"mod_dest_return", catTerminate}, {"return_destination_index", "1"}, {"custom_category", catCustom},
		{"class", "custom_app"}, {"method", "put"}, {"mode", "add"}}
}

func (cp *CallbackPortal) createCallbackApp(ctx context.Context, code string) error {
	_, err := cp.pt.call(ctx, customAppFields(code, callbackDesc, catCustom, "", callbackTarget))
	return err
}

func (cp *CallbackPortal) createPrioApp(ctx context.Context, code string, q CallbackQueue, pr CallbackPriority) error {
	_, err := cp.pt.call(ctx, customAppFields(code, "Callback "+q.Extension, catQueuePrio, strconv.Itoa(pr.ID), ""))
	return err
}

func (cp *CallbackPortal) createCallbackIVR(ctx context.Context, key string, app CallbackApp) error {
	_, err := cp.pt.call(ctx, [][2]string{{"description", callbackDesc}, {"class_of_service_id", ""}, {"ivr_current_cos", ""},
		{"invalid_tries", "3"}, {"welcome_msg_id", ""}, {"instructions_msg_id", ""}, {"invalid_retry_msg_id", ""}, {"invalid_msg_id", ""},
		{"response_timeout", "10"}, {"digit_timeout", "2"}, {"timeout_tries", "3"}, {"timeout_retry_msg_id", ""}, {"timeout_msg_id", ""},
		{"timeout_add_msg", "yes"}, {"invalid_add_msg", "yes"},
		{"mod_invalid", catTerminate}, {"invalid_index", "1"}, {"mod_timeout", catTerminate}, {"timeout_index", "1"},
		{"entries[{{row-count-placeholder}}][option]", ""}, {"entries[{{row-count-placeholder}}][mod_dest]", ""},
		{"entries[{{row-count-placeholder}}][dest]", ""}, {"entries[{{row-count-placeholder}}][enabled]", "1"},
		{"entries[0][option]", key}, {"entries[0][mod_dest]", catCustomApp}, {"entries[0][dest]", strconv.Itoa(app.ID)}, {"entries[0][enabled]", "1"},
		{"class", "ivr"}, {"method", "put"}, {"mode", "add"}})
	return err
}

func (cp *CallbackPortal) createPriority(ctx context.Context, q CallbackQueue, priority int) error {
	_, err := cp.pt.call(ctx, [][2]string{{"description", "Callback " + q.Extension}, {"priority", strconv.Itoa(priority)},
		{"mod_dest", catQueue}, {"destination_index", strconv.Itoa(q.ID)},
		{"class", "queues_priority"}, {"method", "put"}, {"mode", "add"}})
	return err
}

// setQueue is a read-modify-write of the queue's own edit form: every field comes back as the portal
// served it except ivr_id (0 = none) and the membergosub custom row (gosub "" removes it).
func (cp *CallbackPortal) setQueue(ctx context.Context, q CallbackQueue, ivrID int, gosub string) error {
	id := strconv.Itoa(q.ID)
	r, err := cp.pt.request(ctx, "queues", "getContent", "edit", id)
	if err != nil {
		return err
	}
	f, err := formFields(r.HTML, "queues_form")
	if err != nil {
		return err
	}
	ext, qid := "", id
	for _, kv := range f {
		switch kv[0] {
		case "extension":
			ext = kv[1]
		case "queue_id":
			qid = kv[1]
		}
	}
	if !hasField(f, "ivr_id") || ext != q.Extension || qid != id {
		return fmt.Errorf("el formulario del portal no corresponde a la cola %s: no se guardó nada", q.Extension)
	}
	ivr := ""
	if ivrID != 0 {
		ivr = strconv.Itoa(ivrID)
	}
	f = setField(f, "ivr_id", ivr)
	f = setGosub(f, gosub)
	if !hasField(f, "queue_id") {
		f = append(f, [2]string{"queue_id", id})
	}
	f = setField(setField(setField(f, "class", "queues"), "method", "put"), "mode", "edit")
	_, err = cp.pt.call(ctx, f)
	return err
}

func hasField(f [][2]string, name string) bool {
	for _, kv := range f {
		if kv[0] == name {
			return true
		}
	}
	return false
}

// setField replaces the value of every field called name, or appends it when the form has none.
func setField(f [][2]string, name, val string) [][2]string {
	if !hasField(f, name) {
		return append(f, [2]string{name, val})
	}
	for i := range f {
		if f[i][0] == name {
			f[i][1] = val
		}
	}
	return f
}

// setGosub sets, adds (after the existing custom rows) or, for "", removes the membergosub name/value pair.
func setGosub(f [][2]string, gosub string) [][2]string {
	var names, vals []int // positions of the k-th advanced_name[] / advanced_value[]
	for i, kv := range f {
		switch kv[0] {
		case gosubNameField:
			names = append(names, i)
		case gosubValField:
			vals = append(vals, i)
		}
	}
	for k, ni := range names {
		if f[ni][1] != "membergosub" || k >= len(vals) {
			continue
		}
		if gosub != "" {
			f[vals[k]][1] = gosub
			return f
		}
		if f[vals[k]][1] != memberGosub {
			return f // a foreign membergosub is not F-03's to remove
		}
		vi := vals[k]
		out := make([][2]string, 0, len(f))
		for i, kv := range f {
			if i != ni && i != vi {
				out = append(out, kv)
			}
		}
		return out
	}
	if gosub == "" {
		return f
	}
	at := len(f)
	if len(vals) > 0 {
		at = vals[len(vals)-1] + 1
	}
	pair := [][2]string{{gosubNameField, "membergosub"}, {gosubValField, gosub}}
	return append(f[:at], append(pair, f[at:]...)...)
}

func (cp *CallbackPortal) deleteApp(ctx context.Context, id int) error {
	return cp.del(ctx, "custom_app", id)
}
func (cp *CallbackPortal) deleteIVR(ctx context.Context, id int) error { return cp.del(ctx, "ivr", id) }
func (cp *CallbackPortal) deletePriority(ctx context.Context, id int) error {
	return cp.del(ctx, "queues_priority", id)
}

func (cp *CallbackPortal) del(ctx context.Context, class string, id int) error {
	sid := strconv.Itoa(id)
	r, err := cp.pt.request(ctx, class, "delete", "delete", sid)
	if err == nil {
		return nil // a portal version that deletes directly
	}
	var dialog string
	if json.Unmarshal(r.Dialog, &dialog) != nil || strings.TrimSpace(dialog) != "modal-module-delete" {
		return err // e.g. modal-module-errors: still in use
	}
	f, ferr := formFields(r.HTML, "modal-form-delete")
	if ferr != nil {
		return fmt.Errorf("el portal pidió confirmar un borrado inesperado: %w", ferr)
	}
	want := map[string]string{"class": class, "method": "delete", "mode": "deleteConfirmed", "data": sid}
	for k, v := range want {
		if !hasFieldValue(f, k, v) {
			return fmt.Errorf("el portal pidió confirmar un borrado inesperado: falta %s=%s en la confirmación", k, v)
		}
	}
	_, err = cp.pt.call(ctx, f)
	return err
}

// hasFieldValue: name occurs exactly once and holds val (a duplicate could smuggle a second value).
func hasFieldValue(f [][2]string, name, val string) bool {
	n, ok := 0, false
	for _, kv := range f {
		if kv[0] == name {
			n, ok = n+1, kv[1] == val
		}
	}
	return n == 1 && ok
}

// formFields returns what a browser's new FormData(form) would send for the form with id formID, in
// document order: named controls that are not disabled, without unchecked checkboxes/radios, buttons and files.
func formFields(doc, formID string) ([][2]string, error) {
	root, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return nil, err
	}
	var form *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		if form != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "form" && attr(n, "id") == formID {
			form = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(root)
	if form == nil {
		return nil, fmt.Errorf("el portal no devolvió el formulario «%s»", formID)
	}
	var out [][2]string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && attr(n, "name") != "" && !hasAttr(n, "disabled") {
			name := attr(n, "name")
			switch n.Data {
			case "input":
				switch strings.ToLower(attr(n, "type")) {
				case "submit", "button", "file", "image", "reset":
				case "checkbox", "radio":
					if hasAttr(n, "checked") {
						v := attr(n, "value")
						if !hasAttr(n, "value") {
							v = "on"
						}
						out = append(out, [2]string{name, v})
					}
				default:
					out = append(out, [2]string{name, attr(n, "value")})
				}
			case "select":
				for _, v := range selectedValues(n) {
					out = append(out, [2]string{name, v})
				}
				return
			case "textarea":
				out = append(out, [2]string{name, nodeText(n)})
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(form)
	return out, nil
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

// selectedValues is what a browser submits for a <select>: every selected option for a multiple one;
// for a single one the last selected option, or the first non-disabled one when none is. Disabled options never go.
func selectedValues(sel *html.Node) []string {
	var opts []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "option" {
			opts = append(opts, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(sel)
	val := func(o *html.Node) string {
		if hasAttr(o, "value") {
			return attr(o, "value")
		}
		return strings.Join(strings.Fields(nodeText(o)), " ")
	}
	var out []string
	if hasAttr(sel, "multiple") {
		for _, o := range opts {
			if hasAttr(o, "selected") && !hasAttr(o, "disabled") {
				out = append(out, val(o))
			}
		}
		return out
	}
	var chosen *html.Node
	for _, o := range opts {
		if hasAttr(o, "selected") {
			chosen = o
		}
	}
	if chosen == nil {
		for _, o := range opts {
			if !hasAttr(o, "disabled") {
				chosen = o
				break
			}
		}
	}
	if chosen == nil || hasAttr(chosen, "disabled") {
		return nil
	}
	return []string{val(chosen)}
}
