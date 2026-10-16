package pbx

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCallbackDialplanReference(t *testing.T) {
	want, err := os.ReadFile("testdata/callback_default.conf")
	if err != nil {
		t.Fatal(err)
	}
	got := CallbackDialplan(DefaultCallbackConfig(), "", []CallbackRoute{{Queue: "7599", App: "797"}})
	if got != string(want) {
		t.Fatalf("generated file differs from testdata/callback_default.conf:\n%s", got)
	}
}

func TestCallbackDialplanOptions(t *testing.T) {
	c := DefaultCallbackConfig()
	c.Prefix, c.CallerName, c.RingSecs = "9", "Devolucion", 45
	play := "/var/lib/ombutel/static/abc123/recordings/c81e728d9d4c2f636f067f89cc14862c"
	got := CallbackDialplan(c, play, []CallbackRoute{{"7510", "797"}, {"75102", "796"}})
	for _, s := range []string{
		"Channel: Local/9${num}@cos-all",
		"WaitTime: 45",
		`__ORIGINAL_INTERNAL_CID="Devolucion" <${num}>`,
		" same => n,Playback(" + play + ")\n same => n(done),Hangup()",
		`GotoIf($["${QUEUE_NUMBER}"="7510"]?cos-all,797,1)` + "\n" + ` same => n,GotoIf($["${QUEUE_NUMBER}"="75102"]?cos-all,796,1)`,
	} {
		if !strings.Contains(got, s) {
			t.Errorf("missing %q", s)
		}
	}
	if strings.ContainsAny(got, "“”‘’") {
		t.Error("typographic quotes break the shell (the article's bug)")
	}
}

func TestCallbackConfigValidate(t *testing.T) {
	if err := DefaultCallbackConfig().Validate(); err != nil {
		t.Fatalf("defaults must be valid: %v", err)
	}
	bad := map[string]func(*CallbackConfig){
		"key two digits":   func(c *CallbackConfig) { c.Key = "01" },
		"key letter":       func(c *CallbackConfig) { c.Key = "a" },
		"app code letters": func(c *CallbackConfig) { c.AppCode = "79a" },
		"app code long":    func(c *CallbackConfig) { c.AppCode = "123456789" },
		"priority 0":       func(c *CallbackConfig) { c.Priority = 0 },
		"retries 11":       func(c *CallbackConfig) { c.Retries = 11 },
		"retry 30 s":       func(c *CallbackConfig) { c.RetrySecs = 30 },
		"ring 5 s":         func(c *CallbackConfig) { c.RingSecs = 5 },
		"name quote":       func(c *CallbackConfig) { c.CallerName = `Call "Back"` },
		"name accent":      func(c *CallbackConfig) { c.CallerName = "Devolución" },
		"name empty":       func(c *CallbackConfig) { c.CallerName = "" },
		"prefix shell":     func(c *CallbackConfig) { c.Prefix = "9;rm" },
		"prefix long":      func(c *CallbackConfig) { c.Prefix = "1234567" },
		"recording neg":    func(c *CallbackConfig) { c.Recording = -1 },
	}
	for name, mod := range bad {
		c := DefaultCallbackConfig()
		mod(&c)
		if c.Validate() == nil {
			t.Errorf("%s: must be refused", name)
		}
	}
	for _, n := range []string{"", "12", "5555a234", "123456789012345678901", "$(id)"} {
		if CheckDialNumber(n) == nil {
			t.Errorf("number %q must be refused", n)
		}
	}
	if err := CheckDialNumber("55551234"); err != nil {
		t.Fatal(err)
	}
}

func TestParseCallbackConfig(t *testing.T) {
	c := DefaultCallbackConfig()
	c.Prefix, c.Recording = "9", 3
	got, ok := ParseCallbackConfig(CallbackDialplan(c, "", nil))
	if !ok || got != c {
		t.Fatalf("round trip: %+v %v", got, ok)
	}
	for name, file := range map[string]string{
		"missing file":   "",
		"no header":      "[create_callback]\nexten => s,1,NoOp()\n",
		"broken json":    "; f03-config: {\"key\":\n",
		"invalid values": "; f03-config: {\"key\":\"0\",\"appCode\":\"799\",\"priority\":10,\"retries\":2,\"retrySecs\":1800,\"ringSecs\":30,\"callerName\":\"x;y\",\"prefix\":\"\",\"recording\":0}\n",
	} {
		if _, ok := ParseCallbackConfig(file); ok {
			t.Errorf("%s: must not parse", name)
		}
	}
}

// cannedState is what the state script prints for: CC present; queues 7510 (no IVR), 7599 (attached);
// callback app 799 (id 5), priority app 797 (id 6) → priority 3 → queue 7599 (id 9); IVR 11 key 0 → app 5;
// one recording; the file installed; one waiting callback on 7599; codes 799 and 797 answered by Asterisk.
func cannedState(file string) string {
	b64 := base64.StdEncoding.EncodeToString([]byte(file))
	return "@@f03 modules\nname\nsupervision\n" +
		"@@f03 queues\nid\textension\tdescription\tivr_id\tivr_name\tmembergosub\n" +
		"1\t7510\tCola CC\t0\t\t\n9\t7599\tPRUEBA CALLBACK\t11\tCallback\tcheck_position\n" +
		"@@f03 apps\nid\textension\tdescription\tdest_kind\tdest_index\tcustom\n" +
		"5\t799\tCallback\tcustom\t2\tcreate_callback,s,1\n6\t797\tCallback 7599\tqueue_priority\t3\t\n" +
		"@@f03 ivrs\nid\tdescription\toption\tenabled\tdest_kind\tdest_index\n" +
		"11\tCallback\t0\tyes\tcustom_application\t5\n2\tPRINCIPAL\t1\tyes\tivr\t12\n2\tPRINCIPAL\t2\tyes\tivr\t4\n" +
		"@@f03 priorities\nid\tdescription\tpriority\tdest_kind\tdest_index\n3\tCallback 7599\t10\tqueue\t9\n" +
		"@@f03 recordings\nid\tname\n2\tCALL CENTER NUEVO\n" +
		"@@f03 recdir\n/var/lib/ombutel/static/abc123/recordings\n" +
		"@@f03 file\n1\n" + b64 + "\n" +
		"@@f03 log\nasterisk\n" +
		"@@f03 contexts\ncreate_callback\ncheck_position\ncallback_out\n" +
		"@@f03 taken\n799\n797\n" +
		"@@f03 waiting\n/abc123/callback/7599/1760550000.12          : 55551234\n1 results found.\n"
}

func TestParseCallbackState(t *testing.T) {
	file := CallbackDialplan(DefaultCallbackConfig(), "", []CallbackRoute{{"7599", "797"}})
	st, err := parseCallbackState(cannedState(file))
	if err != nil {
		t.Fatal(err)
	}
	if !st.HasCC || len(st.Queues) != 2 || st.Queues[1] != (CallbackQueue{ID: 9, Extension: "7599", Description: "PRUEBA CALLBACK", IVRID: 11, IVRName: "Callback", MemberGosub: "check_position"}) {
		t.Fatalf("queues: %+v", st.Queues)
	}
	if len(st.Apps) != 2 || st.Apps[0].Custom != "create_callback,s,1" || st.Apps[1].DestKind != "queue_priority" || st.Apps[1].DestIndex != 3 {
		t.Fatalf("apps: %+v", st.Apps)
	}
	if len(st.IVRs) != 2 || len(st.IVRs[0].Entries) != 1 || len(st.IVRs[1].Entries) != 2 || st.IVRs[0].Entries[0] != (CallbackIVREntry{Option: "0", Enabled: true, DestKind: "custom_application", DestIndex: 5}) {
		t.Fatalf("ivrs grouped by id in order: %+v", st.IVRs)
	}
	if len(st.Priorities) != 1 || st.Priorities[0].Priority != 10 || len(st.Recordings) != 1 || st.RecordingDir != "/var/lib/ombutel/static/abc123/recordings" {
		t.Fatalf("priorities/recordings: %+v %+v %q", st.Priorities, st.Recordings, st.RecordingDir)
	}
	if !st.FileExists || st.File != file || st.LogOwner != "asterisk" || st.Contexts != 3 {
		t.Fatalf("file/log/contexts: %v %q %d", st.FileExists, st.LogOwner, st.Contexts)
	}
	if !st.TakenCodes["799"] || !st.TakenCodes["797"] || st.TakenCodes["796"] {
		t.Fatalf("taken: %v", st.TakenCodes)
	}
	if len(st.Waiting) != 1 || st.Waiting[0] != (CallbackWaiting{Queue: "7599", ID: "1760550000.12", Number: "55551234"}) {
		t.Fatalf("waiting: %+v", st.Waiting)
	}
}

func TestParseCallbackStateEmptyPBX(t *testing.T) {
	st, err := parseCallbackState("@@f03 modules\n@@f03 queues\n@@f03 apps\n@@f03 ivrs\n@@f03 priorities\n@@f03 recordings\n@@f03 recdir\n@@f03 file\n@@f03 log\n@@f03 contexts\n@@f03 taken\n@@f03 waiting\n")
	if err != nil || st.HasCC || st.FileExists || st.LogOwner != "" || st.Contexts != 0 || len(st.Queues) != 0 {
		t.Fatalf("a PBX without CC or callback: %+v %v", st, err)
	}
	if _, err := parseCallbackState("Permission denied\n"); err == nil {
		t.Fatal("output without the sections must be an error, not an empty state")
	}
}

func TestParseCallbackStateFailures(t *testing.T) {
	if _, err := parseCallbackState(cannedState("") + "@@f03 error apps\n"); err == nil {
		t.Fatal("a failed query must be an error")
	}
	st, err := parseCallbackState(strings.Replace(cannedState(""), ": 55551234", ": 55x51234", 1))
	if err != nil || len(st.Waiting) != 0 {
		t.Fatalf("non-digit number must not parse: %+v %v", st.Waiting, err)
	}
	if _, err := parseCallbackState("@@f03 file\n1\n!!!\n@@f03 waiting\n"); err == nil {
		t.Fatal("invalid base64 must be an error")
	}
	s := callbackStateScript([]string{"799"})
	for _, w := range []string{`if [ -n "$i" ]; then`, "|| echo '@@f03 error queues'"} {
		if !strings.Contains(s, w) {
			t.Errorf("script lacks %q", w)
		}
	}
}

func TestReadCallbackStateOneSession(t *testing.T) {
	var cmds []string
	var script string
	addr, fp := fakeSSH(t, func(cmd string, in []byte) (string, string, int) {
		cmds, script = append(cmds, cmd), string(in)
		return cannedState(""), "", 0
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := ReadCallbackState(ctx, sshProfile(t, addr, fp), Secrets{SSH: "pw"}, []string{"799", "797"}); err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 1 || cmds[0] != "bash -s" {
		t.Fatalf("one SSH session running the script from stdin, got %q", cmds)
	}
	for _, s := range []string{"for c in 799 797; do", "<<'SQL'", "dialplan show $c@cos-all-local", "database get pbx instance"} {
		if !strings.Contains(script, s) {
			t.Errorf("script lacks %q", s)
		}
	}
	if _, err := ReadCallbackState(ctx, sshProfile(t, addr, fp), Secrets{SSH: "pw"}, []string{"79;9"}); err == nil {
		t.Fatal("a non-digit code must never reach the script")
	}
}

// freshPBX: CC present, queues 7510 and 75102 with nothing, recording 2, no callback pieces; 799 and 797 free.
func freshPBX() CallbackState {
	return CallbackState{HasCC: true, TakenCodes: map[string]bool{},
		Queues:       []CallbackQueue{{ID: 1, Extension: "7510", Description: "Cola CC"}, {ID: 2, Extension: "75102", Description: "Cola SF"}},
		Recordings:   []CallbackRecording{{ID: 2, Name: "Gracias"}},
		RecordingDir: "/var/lib/ombutel/static/abc123/recordings"}
}

// installed: freshPBX after a full install on 7510 (app 799 id 5, IVR 11, priority 3, priority app 797 id 6).
func installed() CallbackState {
	st := freshPBX()
	st.Apps = []CallbackApp{{ID: 5, Extension: "799", Description: "Callback", DestKind: "custom", Custom: "create_callback,s,1"},
		{ID: 6, Extension: "797", Description: "Callback 7510", DestKind: "queue_priority", DestIndex: 3}}
	st.IVRs = []CallbackIVR{{ID: 11, Description: "Callback", Entries: []CallbackIVREntry{{Option: "0", Enabled: true, DestKind: "custom_application", DestIndex: 5}}}}
	st.Priorities = []CallbackPriority{{ID: 3, Description: "Callback 7510", Priority: 10, DestKind: "queue", DestIndex: 1}}
	st.Queues[0].IVRID, st.Queues[0].IVRName, st.Queues[0].MemberGosub = 11, "Callback", "check_position"
	st.File = CallbackDialplan(DefaultCallbackConfig(), "", []CallbackRoute{{"7510", "797"}})
	st.FileExists, st.LogOwner, st.Contexts = true, "asterisk", 3
	st.TakenCodes = map[string]bool{"799": true, "797": true}
	return st
}

func kinds(p CallbackPlan) string {
	var s []string
	for _, st := range p.Steps {
		s = append(s, st.Kind+":"+st.Status)
	}
	return strings.Join(s, " ")
}

func TestPlanInstallFresh(t *testing.T) {
	p, err := PlanInstall(freshPBX(), DefaultCallbackConfig(), []string{"7510"})
	if err != nil {
		t.Fatal(err)
	}
	if got := kinds(p); got != "file:nuevo log:nuevo app:nuevo ivr:nuevo priority:nuevo prioapp:nuevo queue:nuevo" {
		t.Fatalf("order and statuses: %s", got)
	}
	if p.Steps[5].Code != "797" || p.Steps[2].Code != "799" || len(p.Routes) != 1 || p.Routes[0] != (CallbackRoute{"7510", "797"}) {
		t.Fatalf("codes/routes: %+v %+v", p.Steps, p.Routes)
	}
	if p.Dialplan != CallbackDialplan(DefaultCallbackConfig(), "", p.Routes) {
		t.Fatal("the plan carries the exact file it will write")
	}
}

func TestPlanInstallResumesPartial(t *testing.T) {
	st := installed()
	again, err := PlanInstall(st, DefaultCallbackConfig(), []string{"7510"})
	if err != nil || kinds(again) != "file:omitir log:omitir app:omitir ivr:omitir priority:omitir prioapp:omitir queue:omitir" {
		t.Fatalf("a complete install plans nothing: %s %v", kinds(again), err)
	}
	st.Queues[0].IVRID, st.Queues[0].MemberGosub = 0, "" // stopped before the queue step
	st.Priorities, st.Apps = nil, st.Apps[:1]           // …and before the priority pieces
	st.TakenCodes = map[string]bool{"799": true}         // so 797 is free again and the file stays the same
	p, _ := PlanInstall(st, DefaultCallbackConfig(), []string{"7510"})
	if got := kinds(p); got != "file:omitir log:omitir app:omitir ivr:omitir priority:nuevo prioapp:nuevo queue:nuevo" {
		t.Fatalf("only the missing pieces: %s", got)
	}
}

func TestPlanInstallBlocksForeignQueue(t *testing.T) {
	st := freshPBX()
	st.Queues[0].IVRID, st.Queues[0].IVRName = 2, "PRINCIPAL"
	st.Queues[1].MemberGosub = "otra_cosa"
	p, err := PlanInstall(st, DefaultCallbackConfig(), []string{"7510", "75102"})
	if err != nil {
		t.Fatal(err)
	}
	blocked := 0
	for _, s := range p.Steps {
		if s.Status == CbBlocked {
			blocked++
			if !strings.Contains(s.Error, "PRINCIPAL") && !strings.Contains(s.Error, "otra_cosa") {
				t.Errorf("the reason names what is there: %q", s.Error)
			}
		}
	}
	if blocked != 6 || len(p.Routes) != 0 {
		t.Fatalf("all 3 steps of both queues blocked and no route: %s", kinds(p))
	}
}

func TestQueueViews(t *testing.T) {
	st := installed()
	st.Queues = append(st.Queues, CallbackQueue{ID: 3, Extension: "7600", IVRID: 2, IVRName: "PRINCIPAL"})
	st.Waiting = []CallbackWaiting{{Queue: "7510", ID: "1.1", Number: "1"}}
	v := st.QueueViews()
	if !v[0].Attached || !v[0].Hooked || v[0].Foreign != "" || v[0].Waiting != 1 {
		t.Fatalf("7510 has callback: %+v", v[0])
	}
	if v[1].Attached || v[1].Hooked || v[1].Foreign != "" {
		t.Fatalf("75102 is free: %+v", v[1])
	}
	if !strings.Contains(v[2].Foreign, "PRINCIPAL") {
		t.Fatalf("7600 has a foreign IVR: %+v", v[2])
	}
}

func TestPlanInstallCodeCollision(t *testing.T) {
	st := freshPBX()
	st.TakenCodes = map[string]bool{"797": true}
	p, _ := PlanInstall(st, DefaultCallbackConfig(), []string{"7510", "75102"})
	if p.Steps[5].Code != "796" || p.Steps[8].Code != "795" {
		t.Fatalf("next free codes downward: %+v", p.Steps)
	}
	st.TakenCodes["799"] = true
	p, _ = PlanInstall(st, DefaultCallbackConfig(), []string{"7510"})
	if p.Steps[2].Status != CbBlocked || !strings.Contains(p.Steps[2].Error, "799") {
		t.Fatalf("a taken app code blocks with the code named: %+v", p.Steps[2])
	}
	st = installed() // 799 is taken by our own app: fine
	if p, _ = PlanInstall(st, DefaultCallbackConfig(), []string{"7510"}); p.Steps[2].Status != CbSkip {
		t.Fatalf("our own app is not a collision: %+v", p.Steps[2])
	}
}

func TestPlanInstallMissingRecording(t *testing.T) {
	c := DefaultCallbackConfig()
	c.Recording = 9
	if _, err := PlanInstall(freshPBX(), c, []string{"7510"}); err == nil || !strings.Contains(err.Error(), "grabación") {
		t.Fatalf("a deleted recording must be named: %v", err)
	}
	c.Recording = 2
	p, err := PlanInstall(freshPBX(), c, []string{"7510"})
	if err != nil || !strings.Contains(p.Dialplan, "Playback(/var/lib/ombutel/static/abc123/recordings/c81e728d9d4c2f636f067f89cc14862c)") {
		t.Fatalf("recording 2 plays from md5(\"2\"): %v", err)
	}
}

func TestPlanInstallRefusals(t *testing.T) {
	st := freshPBX()
	for name, f := range map[string]func() error{
		"no CC":         func() error { s := st; s.HasCC = false; _, err := PlanInstall(s, DefaultCallbackConfig(), []string{"7510"}); return err },
		"no queues":     func() error { _, err := PlanInstall(st, DefaultCallbackConfig(), nil); return err },
		"unknown queue": func() error { _, err := PlanInstall(st, DefaultCallbackConfig(), []string{"9999"}); return err },
		"bad config":    func() error { c := DefaultCallbackConfig(); c.Prefix = "x"; _, err := PlanInstall(st, c, []string{"7510"}); return err },
	} {
		if f() == nil {
			t.Errorf("%s: must be refused", name)
		}
	}
}

func TestPlanRemove(t *testing.T) {
	st := installed()
	st.Waiting = []CallbackWaiting{{Queue: "7510", ID: "1760550000.12", Number: "55551234"}}
	p, err := PlanRemove(st, []string{"7510"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := kinds(p); got != "detach:nuevo delprioapp:nuevo delpriority:nuevo dropwaiting:nuevo delivr:nuevo delapp:nuevo delfile:nuevo" {
		t.Fatalf("last queue out removes everything shared: %s", got)
	}
	if p, _ = PlanRemove(st, []string{"7510"}, false); strings.Contains(kinds(p), "dropwaiting") {
		t.Fatal("waiting callbacks are only dropped when asked")
	}
	if _, err := PlanRemove(st, []string{"75102"}, false); err == nil {
		t.Fatal("a queue without callback cannot be removed")
	}
}

func TestPlanRemoveNeedsConfig(t *testing.T) {
	st := installed() // add 75102 as a second attached queue (priority 4, app 796)
	st.Queues[1].IVRID, st.Queues[1].MemberGosub = 11, "check_position"
	st.Priorities = append(st.Priorities, CallbackPriority{ID: 4, Description: "Callback 75102", Priority: 10, DestKind: "queue", DestIndex: 2})
	st.Apps = append(st.Apps, CallbackApp{ID: 7, Extension: "796", Description: "Callback 75102", DestKind: "queue_priority", DestIndex: 4})
	p, err := PlanRemove(st, []string{"7510"}, false)
	if err != nil || kinds(p) != "detach:nuevo delprioapp:nuevo delpriority:nuevo file:nuevo" || len(p.Routes) != 1 || p.Routes[0].Queue != "75102" {
		t.Fatalf("75102 keeps callback, the file loses 7510's line: %s %+v %v", kinds(p), p.Routes, err)
	}
	st.File = "; edited by hand\n"
	if _, err := PlanRemove(st, []string{"7510"}, false); err == nil || !strings.Contains(err.Error(), "instálelo de nuevo") {
		t.Fatalf("no settings header: must say to reinstall, got %v", err)
	}
}

func TestPlanRoutesDigitsOnly(t *testing.T) {
	st := freshPBX()
	st.Queues[0].Extension = "75x0"
	p, err := PlanInstall(st, DefaultCallbackConfig(), []string{"75x0"})
	if err != nil || len(p.Routes) != 0 || p.Steps[4].Status != CbBlocked || !strings.Contains(p.Steps[4].Error, "75x0") {
		t.Fatalf("a non-digit queue is blocked, never routed: %s %+v %v", kinds(p), p.Routes, err)
	}
	st = installed()
	st.Queues[1].IVRID, st.Queues[1].MemberGosub = 11, "check_position"
	st.Priorities = append(st.Priorities, CallbackPriority{ID: 4, Description: "Callback 75102", Priority: 10, DestKind: "queue", DestIndex: 2})
	st.Apps = append(st.Apps, CallbackApp{ID: 7, Extension: "79x", Description: "Callback 75102", DestKind: "queue_priority", DestIndex: 4})
	if _, err := PlanRemove(st, []string{"7510"}, false); err == nil || !strings.Contains(err.Error(), "79x") {
		t.Fatalf("a kept route with a non-digit code is an error: %v", err)
	}
	if _, err := PlanInstall(st, DefaultCallbackConfig(), []string{"7510"}); err == nil || !strings.Contains(err.Error(), "código no numérico") {
		t.Fatalf("install keeps the same route and refuses it too: %v", err)
	}
	st = installed()
	st.Apps[1].Extension = "79x"
	p, err = PlanInstall(st, DefaultCallbackConfig(), []string{"7510"})
	if err != nil || p.Steps[5].Status != CbBlocked || !strings.Contains(p.Steps[5].Error, "aplicación de prioridad «79x»") || len(p.Routes) != 0 {
		t.Fatalf("a selected queue with a non-digit priority app is blocked: %s %+v %v", kinds(p), p.Steps[5], err)
	}
}

func TestPlanQueuesDedupedAndForeignIVRKept(t *testing.T) {
	p, err := PlanInstall(freshPBX(), DefaultCallbackConfig(), []string{"75102", "7510", "75102"})
	if err != nil || strings.Join(p.Queues, ",") != "75102,7510" {
		t.Fatalf("queues deduped in first-seen order: %v %v", p.Queues, err)
	}
	st := freshPBX()
	st.Queues[0].IVRID, st.Queues[0].IVRName, st.Queues[0].MemberGosub = 2, "PRINCIPAL", "check_position"
	r, err := PlanRemove(st, []string{"7510", "7510"}, false)
	if err != nil || len(r.Queues) != 1 || r.Steps[0].Kind != StepDetach || r.Steps[0].After != "IVR: PRINCIPAL (se mantiene) · membergosub: ninguno" {
		t.Fatalf("a foreign IVR is kept and the plan says so: %+v %v", r, err)
	}
}

func TestRunCallbackNeverRunsBlocked(t *testing.T) {
	plan := CallbackPlan{Steps: []CallbackStep{{Kind: StepApp, Status: CbNew}, {Kind: StepQueue, Status: CbBlocked, Error: "la cola ya tiene el IVR «X»"}, {Kind: StepIVR, Status: CbNew}}}
	var ran []string
	got := RunCallback(context.Background(), plan, func(_ context.Context, s CallbackStep) error { ran = append(ran, s.Kind); return nil }, func(int) {})
	if strings.Join(ran, ",") != "app" || got[0].Status != CbApplied || got[1].Status != CbNotRun || got[1].Error != "la cola ya tiene el IVR «X»" || got[2].Status != CbNotRun {
		t.Fatalf("blocked steps never run and stop the rest: %v %+v", ran, got)
	}
}

func TestVerifyDelPrioAppByCode(t *testing.T) {
	st := installed()
	st.Priorities = nil // the priority is already gone; the app with its code is still there
	steps := []CallbackStep{{Kind: StepDelPrioApp, Queue: "7510", Code: "797", Status: CbApplied}}
	if got := VerifyCallback(CallbackPlan{Remove: true}, steps, st); got[0].Status != CbFailed {
		t.Fatalf("the app is still there: %+v", got[0])
	}
	st.Apps = st.Apps[:1]
	if got := VerifyCallback(CallbackPlan{Remove: true}, steps, st); got[0].Status != CbApplied {
		t.Fatalf("the app is gone: %+v", got[0])
	}
}

func TestVerifyCallback(t *testing.T) {
	p, _ := PlanInstall(freshPBX(), DefaultCallbackConfig(), []string{"7510"})
	steps := append([]CallbackStep(nil), p.Steps...)
	for i := range steps {
		steps[i].Status = CbApplied
	}
	got := VerifyCallback(p, steps, installed())
	for _, s := range got {
		if s.Status != CbApplied {
			t.Fatalf("a complete PBX verifies every step: %+v", s)
		}
	}
	missing := installed()
	missing.Queues[0].MemberGosub = ""
	got = VerifyCallback(p, steps, missing)
	if got[6].Status != CbFailed || !strings.Contains(got[6].Error, "después de aplicar") {
		t.Fatalf("the queue step fails on the read-back: %+v", got[6])
	}
}

func TestRunCallbackStopsAtFirstFailure(t *testing.T) {
	plan := CallbackPlan{Steps: []CallbackStep{{Kind: StepFile, Status: CbSkip}, {Kind: StepApp, Status: CbNew}, {Kind: StepIVR, Status: CbNew}, {Kind: StepQueue, Status: CbNew}}}
	var ran []string
	var done []int
	got := RunCallback(context.Background(), plan, func(_ context.Context, s CallbackStep) error {
		ran = append(ran, s.Kind)
		if s.Kind == StepIVR {
			return errors.New("el portal rechazó el cambio: X")
		}
		return nil
	}, func(n int) { done = append(done, n) })
	if strings.Join(ran, ",") != "app,ivr" {
		t.Fatalf("skipped steps never run, nothing after a failure: %v", ran)
	}
	if strings.Join([]string{got[0].Status, got[1].Status, got[2].Status, got[3].Status}, " ") != "omitido aplicado fallo no_ejecutado" || len(done) != 4 {
		t.Fatalf("stamps: %+v progress %v", got, done)
	}
}
