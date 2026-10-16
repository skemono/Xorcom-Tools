package pbx

import (
	"context"
	"errors"
	"fmt"
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

// queueSnapshot is one queue's row plus its CUSTOM params ("param:<name>"), compared around a save.
func queueSnapshot(ctx context.Context, p Profile, sec Secrets, id int) (map[string]string, error) {
	rows, err := MySQL(ctx, p, sec, fmt.Sprintf("SELECT * FROM ombu_queues WHERE queue_id = %d;", id))
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, fmt.Errorf("la cola %d no existe", id)
	}
	snap := rows[0]
	params, err := MySQL(ctx, p, sec, fmt.Sprintf("SELECT name, value FROM ombu_queue_params WHERE queue_id = %d;", id))
	if err != nil {
		return nil, err
	}
	for _, r := range params {
		snap["param:"+r["name"]] = r["value"]
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
	if err := save(); err != nil {
		return err
	}
	after, err := queueSnapshot(ctx, cp.p, cp.sec, q.ID)
	if err != nil {
		return fmt.Errorf("no se pudo verificar la cola %s después de guardar: %w", q.Extension, err)
	}
	if d := queueChanges(before, after); len(d) > 0 {
		return fmt.Errorf("el portal cambió otros campos de la cola %s (revíselos en el portal): %v", q.Extension, d)
	}
	return nil
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
			return cp.guardQueue(ctx, *q, func() error { return cp.setQueue(ctx, *q, iv.ID, memberGosub) })
		case StepDetach:
			keep := 0 // a foreign IVR stays; only membergosub goes
			if iv := st.CallbackIVR(); q.IVRID != 0 && (iv == nil || q.IVRID != iv.ID) {
				keep = q.IVRID
			}
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
	ivr := ""
	if ivrID != 0 {
		ivr = strconv.Itoa(ivrID)
	}
	f = setField(f, "ivr_id", ivr)
	f = setGosub(f, gosub)
	if !hasField(f, "queue_id") {
		f = append(f, [2]string{"queue_id", id})
	}
	_, err = cp.pt.call(ctx, append(f, [2]string{"class", "queues"}, [2]string{"method", "put"}, [2]string{"mode", "edit"}))
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
	_, err := cp.pt.request(ctx, class, "delete", "delete", strconv.Itoa(id))
	return err
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
				if v, ok := selectedValue(n); ok {
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

// selectedValue is the selected option's value, or the first option's when none is selected.
func selectedValue(sel *html.Node) (string, bool) {
	var first, chosen *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "option" {
			if first == nil {
				first = n
			}
			if chosen == nil && hasAttr(n, "selected") {
				chosen = n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(sel)
	if chosen == nil {
		chosen = first
	}
	if chosen == nil {
		return "", false
	}
	if hasAttr(chosen, "value") {
		return attr(chosen, "value"), true
	}
	return strings.TrimSpace(nodeText(chosen)), true
}
