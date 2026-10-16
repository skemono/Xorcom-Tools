package pbx

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
)

// Statuses: before Aplicar/Quitar (nuevo/omitir/bloqueado) and after (aplicado/omitido/fallo/no_ejecutado).
const (
	CbNew     = "nuevo"
	CbSkip    = "omitir"
	CbBlocked = "bloqueado"
	CbApplied = "aplicado"
	CbSkipped = "omitido"
	CbFailed  = "fallo"
	CbNotRun  = "no_ejecutado"
)

// Step kinds. Install runs file, log, app, ivr, then per queue priority, prioapp, queue (the step that
// touches live calls last). Quitar runs per queue detach, delprioapp, delpriority, dropwaiting, then the shared pieces.
const (
	StepFile        = "file"
	StepLog         = "log"
	StepApp         = "app"
	StepIVR         = "ivr"
	StepPriority    = "priority"
	StepPrioApp     = "prioapp"
	StepQueue       = "queue"
	StepDetach      = "detach"
	StepDelPrioApp  = "delprioapp"
	StepDelPriority = "delpriority"
	StepDropWaiting = "dropwaiting"
	StepDelIVR      = "delivr"
	StepDelApp      = "delapp"
	StepDelFile     = "delfile"
)

const (
	callbackTarget = "create_callback,s,1"
	memberGosub    = "check_position"
	firstPrioCode  = 797
	lastPrioCode   = 748
)

// CallbackStep is one line of the preview and of the result.
type CallbackStep struct {
	Kind   string `json:"kind"`
	Queue  string `json:"queue"` // queue extension, for per-queue steps
	Code   string `json:"code"`  // app code, for app steps
	Label  string `json:"label"`
	Before string `json:"before"`
	After  string `json:"after"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

// CallbackPlan is exactly what Aplicar/Quitar will do: the steps in run order and the file content.
type CallbackPlan struct {
	Remove      bool            `json:"remove"`
	DropWaiting bool            `json:"dropWaiting"`
	Config      CallbackConfig  `json:"config"`
	Queues      []string        `json:"queues"`
	Routes      []CallbackRoute `json:"routes"`
	Dialplan    string          `json:"dialplan"`
	Steps       []CallbackStep  `json:"steps"`
}

// CandidateCodes are the dial codes ReadCallbackState checks: the callback app's, then the priority range.
func CandidateCodes(appCode string) []string {
	codes := []string{appCode}
	for n := firstPrioCode; n >= lastPrioCode; n-- {
		if c := strconv.Itoa(n); c != appCode {
			codes = append(codes, c)
		}
	}
	return codes
}

var rePlayback = regexp.MustCompile(`^/var/lib/ombutel/static/[A-Za-z0-9]+/recordings/[0-9a-f]{32}$`)

// RecordingPath is where the IVRs play recording id from (the portal names it md5 of the id), or "".
func (st CallbackState) RecordingPath(id int) string {
	if id == 0 || st.RecordingDir == "" {
		return ""
	}
	for _, r := range st.Recordings {
		if r.ID == id {
			path := fmt.Sprintf("%s/%x", st.RecordingDir, md5.Sum([]byte(strconv.Itoa(id))))
			if rePlayback.MatchString(path) {
				return path
			}
		}
	}
	return ""
}

// CallbackApp is F-03's custom app: the one whose destination is Custom create_callback,s,1.
func (st CallbackState) CallbackApp() *CallbackApp {
	for i := range st.Apps {
		if st.Apps[i].DestKind == "custom" && st.Apps[i].Custom == callbackTarget {
			return &st.Apps[i]
		}
	}
	return nil
}

// CallbackIVR is F-03's breakout IVR: one entry, going to the callback app.
func (st CallbackState) CallbackIVR() *CallbackIVR {
	app := st.CallbackApp()
	if app == nil {
		return nil
	}
	for i := range st.IVRs {
		e := st.IVRs[i].Entries
		if len(e) == 1 && e[0].DestKind == "custom_application" && e[0].DestIndex == app.ID {
			return &st.IVRs[i]
		}
	}
	return nil
}

func (st CallbackState) Queue(ext string) *CallbackQueue {
	for i := range st.Queues {
		if st.Queues[i].Extension == ext {
			return &st.Queues[i]
		}
	}
	return nil
}

// Priority is queue q's callback Queue Priority: "Callback <ext>", going to q.
func (st CallbackState) Priority(q CallbackQueue) *CallbackPriority {
	for i := range st.Priorities {
		p := st.Priorities[i]
		if p.DestKind == "queue" && p.DestIndex == q.ID && p.Description == "Callback "+q.Extension {
			return &st.Priorities[i]
		}
	}
	return nil
}

// PrioApp is the custom app going to Queue Priority pr.
func (st CallbackState) PrioApp(pr *CallbackPriority) *CallbackApp {
	if pr == nil {
		return nil
	}
	for i := range st.Apps {
		if st.Apps[i].DestKind == "queue_priority" && st.Apps[i].DestIndex == pr.ID {
			return &st.Apps[i]
		}
	}
	return nil
}

// Attached: q breaks out to the callback IVR and runs check_position.
func (st CallbackState) Attached(q CallbackQueue) bool {
	iv := st.CallbackIVR()
	return iv != nil && q.IVRID == iv.ID && q.MemberGosub == memberGosub
}

// hooked: q carries any part of F-03 (the callback IVR or check_position), so Quitar must detach it.
func (st CallbackState) hooked(q CallbackQueue) bool {
	iv := st.CallbackIVR()
	return iv != nil && q.IVRID == iv.ID || q.MemberGosub == memberGosub
}

// foreign is why F-03 must not touch q ("" = it may): an IVR or a membergosub that is not F-03's.
func (st CallbackState) foreign(q CallbackQueue) string {
	ivr := st.CallbackIVR()
	switch {
	case q.IVRID != 0 && (ivr == nil || q.IVRID != ivr.ID):
		return fmt.Sprintf("la cola ya tiene el IVR «%s»: F-03 no lo reemplaza", q.IVRName)
	case q.MemberGosub != "" && q.MemberGosub != memberGosub:
		return fmt.Sprintf("la cola ya tiene membergosub «%s»: F-03 no lo reemplaza", q.MemberGosub)
	}
	return ""
}

// CallbackQueueView is one queue as the sheet shows it; the rules stay here, not in the UI.
type CallbackQueueView struct {
	Queue    CallbackQueue `json:"queue"`
	Attached bool          `json:"attached"` // callback fully on
	Hooked   bool          `json:"hooked"`   // any F-03 part on: Quitar can take it off
	Foreign  string        `json:"foreign"`  // why Instalar must skip it, "" = it can take callback
	Waiting  int           `json:"waiting"`
}

func (st CallbackState) QueueViews() []CallbackQueueView {
	out := make([]CallbackQueueView, 0, len(st.Queues))
	for _, q := range st.Queues {
		out = append(out, CallbackQueueView{Queue: q, Attached: st.Attached(q), Hooked: st.hooked(q), Foreign: st.foreign(q), Waiting: st.waiting(q.Extension)})
	}
	return out
}

func (st CallbackState) codeTaken(code string) bool {
	if st.TakenCodes[code] {
		return true
	}
	for _, a := range st.Apps {
		if a.Extension == code {
			return true
		}
	}
	for _, q := range st.Queues {
		if q.Extension == code {
			return true
		}
	}
	return false
}

func (st CallbackState) waiting(queue string) int {
	n := 0
	for _, w := range st.Waiting {
		if w.Queue == queue {
			n++
		}
	}
	return n
}

// step builds a plan line; a block wins over done (leftovers on a foreign queue are not "installed").
func step(kind, queue, code, label, before, after string, done bool, block string) CallbackStep {
	s := CallbackStep{Kind: kind, Queue: queue, Code: code, Label: label, Before: before, After: after, Status: CbNew}
	switch {
	case block != "":
		s.Status, s.Error = CbBlocked, block
	case done:
		s.Status, s.After = CbSkip, "ya instalado: se omite"
	}
	return s
}

func or(s, empty string) string {
	if s == "" {
		return empty
	}
	return s
}

func exists(b bool) string {
	if b {
		return "existe"
	}
	return "no existe"
}

func queueNow(q CallbackQueue) string {
	return fmt.Sprintf("IVR: %s · membergosub: %s", or(q.IVRName, "ninguno"), or(q.MemberGosub, "ninguno"))
}

// fileStep compares the installed file with want. Equal content counts as installed only if Asterisk
// has the 3 contexts loaded; otherwise the step rewrites it and reloads.
func fileStep(st CallbackState, want string) CallbackStep {
	before := "no existe"
	if st.FileExists {
		before = "distinto: se reemplaza"
		if st.File == want {
			before = "igual"
		}
	}
	return step(StepFile, "", "", "Archivo extensions__65-callback.conf", before, "contenido de la vista previa", st.FileExists && st.File == want && st.Contexts == 3, "")
}

// PlanInstall lists every step Aplicar takes to put callback on queues, in run order.
func PlanInstall(st CallbackState, c CallbackConfig, queues []string) (CallbackPlan, error) {
	if !st.HasCC {
		return CallbackPlan{}, errors.New("esta PBX no tiene Call Center: la devolución de llamada necesita el módulo del Call Center")
	}
	if err := c.Validate(); err != nil {
		return CallbackPlan{}, err
	}
	if len(queues) == 0 {
		return CallbackPlan{}, errors.New("elija al menos una cola")
	}
	playback := st.RecordingPath(c.Recording)
	if c.Recording != 0 && playback == "" {
		return CallbackPlan{}, errors.New("la grabación elegida ya no existe en la PBX: vuelva a leer el estado")
	}
	app, ivr := st.CallbackApp(), st.CallbackIVR()
	used := map[string]bool{c.AppCode: true}
	next := firstPrioCode
	take := func() string {
		for ; next >= lastPrioCode; next-- {
			if code := strconv.Itoa(next); !used[code] && !st.codeTaken(code) {
				used[code] = true
				next--
				return code
			}
		}
		return ""
	}
	selected := map[string]bool{}
	var routes []CallbackRoute
	var perQueue []CallbackStep
	for _, ext := range queues {
		q := st.Queue(ext)
		if q == nil {
			return CallbackPlan{}, fmt.Errorf("la cola %s no existe en la PBX", ext)
		}
		if selected[ext] {
			continue
		}
		selected[ext] = true
		block := st.foreign(*q)
		if block == "" && !reDigits.MatchString(ext) {
			block = fmt.Sprintf("la cola «%s» no tiene un número válido", ext)
		}
		pr := st.Priority(*q)
		pa := st.PrioApp(pr)
		code := ""
		switch {
		case pa != nil:
			code = pa.Extension
			if block == "" && !reDigits.MatchString(code) {
				block = fmt.Sprintf("la aplicación de prioridad «%s» no tiene un código válido", code)
			}
		case block == "":
			if code = take(); code == "" {
				block = fmt.Sprintf("no hay códigos libres entre %d y %d para la aplicación de prioridad", firstPrioCode, lastPrioCode)
			}
		}
		if block == "" {
			routes = append(routes, CallbackRoute{Queue: ext, App: code})
		}
		name := "Callback " + ext
		perQueue = append(perQueue,
			step(StepPriority, ext, "", fmt.Sprintf("Prioridad de cola «%s»", name), exists(pr != nil),
				fmt.Sprintf("prioridad %d → cola %s", c.Priority, ext), pr != nil, block),
			step(StepPrioApp, ext, code, fmt.Sprintf("Aplicación %s «%s»", or(code, "—"), name), exists(pa != nil),
				fmt.Sprintf("→ prioridad «%s»", name), pa != nil, block),
			step(StepQueue, ext, "", fmt.Sprintf("Cola %s · %s", ext, q.Description), queueNow(*q),
				"IVR: Callback · membergosub: "+memberGosub, st.Attached(*q), block))
	}
	for _, q := range st.Queues { // queues that already have callback keep their callback_out line
		if !selected[q.Extension] && st.Attached(q) {
			if pa := st.PrioApp(st.Priority(q)); pa != nil {
				if !reDigits.MatchString(q.Extension) || !reDigits.MatchString(pa.Extension) {
					return CallbackPlan{}, nonDigitErr(q.Extension, pa.Extension)
				}
				routes = append(routes, CallbackRoute{Queue: q.Extension, App: pa.Extension})
			}
		}
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].Queue < routes[j].Queue })
	plan := CallbackPlan{Config: c, Queues: queues, Routes: routes, Dialplan: CallbackDialplan(c, playback, routes)}
	appBlock := ""
	if app == nil && st.codeTaken(c.AppCode) {
		appBlock = fmt.Sprintf("el código %s ya está en uso en la PBX: elija otro", c.AppCode)
	}
	appCode := c.AppCode
	if app != nil {
		appCode = app.Extension
	}
	logBefore := "no existe"
	if st.LogOwner != "" {
		logBefore = "dueño " + st.LogOwner
	}
	plan.Steps = append([]CallbackStep{
		fileStep(st, plan.Dialplan),
		step(StepLog, "", "", "Registro /var/log/cpbx/callback.log", logBefore,
			"existe, dueño asterisk", st.LogOwner == "asterisk", ""),
		step(StepApp, "", appCode, fmt.Sprintf("Aplicación %s «Callback»", appCode), exists(app != nil),
			"→ Custom "+callbackTarget, app != nil, appBlock),
		step(StepIVR, "", c.Key, "IVR «Callback»", exists(ivr != nil),
			fmt.Sprintf("tecla %s → aplicación %s", c.Key, appCode), ivr != nil, ""),
	}, perQueue...)
	return plan, nil
}

// nonDigitErr: a queue or priority-app code from the PBX that cannot go into the generated dialplan.
func nonDigitErr(queue, app string) error {
	v := queue
	if reDigits.MatchString(queue) {
		v = app
	}
	return fmt.Errorf("la PBX tiene un código no numérico «%s»: revíselo en el portal", v)
}

// PlanRemove lists every step Quitar takes to take callback off queues; when no queue keeps it, the
// shared IVR, app and file go too. The log is always kept.
func PlanRemove(st CallbackState, queues []string, dropWaiting bool) (CallbackPlan, error) {
	if len(queues) == 0 {
		return CallbackPlan{}, errors.New("elija al menos una cola")
	}
	app, ivr := st.CallbackApp(), st.CallbackIVR()
	selected := map[string]bool{}
	var steps []CallbackStep
	for _, ext := range queues {
		q := st.Queue(ext)
		if q == nil {
			return CallbackPlan{}, fmt.Errorf("la cola %s no existe en la PBX", ext)
		}
		if selected[ext] {
			continue
		}
		pr := st.Priority(*q)
		pa := st.PrioApp(pr)
		hooked := st.hooked(*q)
		if !hooked && pr == nil && pa == nil {
			return CallbackPlan{}, fmt.Errorf("la cola %s no tiene devolución de llamada", ext)
		}
		selected[ext] = true
		if hooked {
			steps = append(steps, step(StepDetach, ext, "", fmt.Sprintf("Cola %s · %s", ext, q.Description), queueNow(*q), "IVR: ninguno · membergosub: ninguno", false, ""))
		}
		if pa != nil {
			steps = append(steps, step(StepDelPrioApp, ext, pa.Extension, fmt.Sprintf("Aplicación %s «%s»", pa.Extension, pa.Description), "existe", "se elimina", false, ""))
		}
		if pr != nil {
			steps = append(steps, step(StepDelPriority, ext, "", fmt.Sprintf("Prioridad de cola «%s»", pr.Description), "existe", "se elimina", false, ""))
		}
		if n := st.waiting(ext); dropWaiting && n > 0 {
			steps = append(steps, step(StepDropWaiting, ext, "", "Devoluciones pendientes de la cola "+ext, fmt.Sprintf("%d pendiente(s)", n), "se descartan", false, ""))
		}
	}
	var routes []CallbackRoute
	users := 0
	for _, q := range st.Queues {
		if selected[q.Extension] || !st.hooked(q) {
			continue
		}
		users++
		if pa := st.PrioApp(st.Priority(q)); pa != nil && st.Attached(q) {
			if !reDigits.MatchString(q.Extension) || !reDigits.MatchString(pa.Extension) {
				return CallbackPlan{}, nonDigitErr(q.Extension, pa.Extension)
			}
			routes = append(routes, CallbackRoute{Queue: q.Extension, App: pa.Extension})
		}
	}
	plan := CallbackPlan{Remove: true, DropWaiting: dropWaiting, Queues: queues, Routes: routes}
	if users == 0 {
		if ivr != nil {
			steps = append(steps, step(StepDelIVR, "", "", "IVR «"+ivr.Description+"»", "existe", "se elimina", false, ""))
		}
		if app != nil {
			steps = append(steps, step(StepDelApp, "", app.Extension, fmt.Sprintf("Aplicación %s «%s»", app.Extension, app.Description), "existe", "se elimina", false, ""))
		}
		if st.FileExists {
			steps = append(steps, step(StepDelFile, "", "", "Archivo extensions__65-callback.conf", "existe", "se elimina", false, ""))
		}
	} else {
		c, ok := ParseCallbackConfig(st.File)
		if !ok {
			return CallbackPlan{}, errors.New("el archivo de devolución de llamada no tiene su configuración (¿editado a mano?): instálelo de nuevo antes de quitar colas")
		}
		plan.Config = c
		plan.Dialplan = CallbackDialplan(c, st.RecordingPath(c.Recording), routes)
		if st.File != plan.Dialplan || st.Contexts != 3 {
			steps = append(steps, fileStep(st, plan.Dialplan))
		}
	}
	plan.Steps = steps
	return plan, nil
}

// VerifyCallback stamps each applied step from the read-back st: a step whose effect is not there fails.
func VerifyCallback(plan CallbackPlan, steps []CallbackStep, st CallbackState) []CallbackStep {
	out := append([]CallbackStep(nil), steps...)
	for i, s := range out {
		if s.Status != CbApplied {
			continue
		}
		q := st.Queue(s.Queue)
		var pr *CallbackPriority
		if q != nil {
			pr = st.Priority(*q)
		}
		ok := true
		switch s.Kind {
		case StepFile:
			ok = st.FileExists && st.File == plan.Dialplan && st.Contexts == 3
		case StepLog:
			ok = st.LogOwner == "asterisk"
		case StepApp:
			ok = st.CallbackApp() != nil
		case StepIVR:
			ok = st.CallbackIVR() != nil
		case StepPriority:
			ok = pr != nil
		case StepPrioApp:
			pa := st.PrioApp(pr)
			ok = pa != nil && pa.Extension == s.Code
		case StepQueue:
			ok = q != nil && st.Attached(*q)
		case StepDetach:
			ok = q != nil && !st.hooked(*q)
		case StepDelPrioApp:
			ok = st.PrioApp(pr) == nil
		case StepDelPriority:
			ok = pr == nil
		case StepDropWaiting:
			ok = st.waiting(s.Queue) == 0
		case StepDelIVR:
			ok = st.CallbackIVR() == nil
		case StepDelApp:
			ok = st.CallbackApp() == nil
		case StepDelFile:
			ok = !st.FileExists && st.Contexts == 0
		}
		if !ok {
			out[i].Status = CbFailed
			if plan.Remove {
				out[i].Error = "sigue en la PBX después de quitar"
			} else {
				out[i].Error = "no se encontró después de aplicar"
			}
		}
	}
	return out
}

// RunCallback runs the plan's steps in order. Steps planned as omitir are not run; the first failure
// stops the run and every later step stamps no_ejecutado. progress gets the number of steps finished.
func RunCallback(ctx context.Context, plan CallbackPlan, do func(context.Context, CallbackStep) error, progress func(done int)) []CallbackStep {
	out := append([]CallbackStep(nil), plan.Steps...)
	failed := false
	for i := range out {
		switch {
		case out[i].Status == CbSkip:
			out[i].Status, out[i].Error = CbSkipped, "ya estaba así"
		case failed:
			out[i].Status, out[i].Error = CbNotRun, "no se ejecutó: falló un paso anterior"
		default:
			if err := do(ctx, out[i]); err != nil {
				out[i].Status, out[i].Error, failed = CbFailed, Describe(err), true
			} else {
				out[i].Status = CbApplied
			}
		}
		progress(i + 1)
	}
	return out
}
