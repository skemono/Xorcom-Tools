package pbx

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
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
		" same => n,Playback(" + play + ")\n same => n(done),Return()",
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
		"@@f03 categories\nid\tname\n25\tcustom\n4\tcustom_application\n10\tqueue\n11\tqueue_priority\n24\tterminate_call\n" +
		"@@f03 rendered\n7599\tIVR-11\tcheck_position\n" +
		"@@f03 recdir\n/var/lib/ombutel/static/abc123/recordings\n" +
		"@@f03 file\n1\n" + b64 + "\n" +
		"@@f03 log\nasterisk\n" +
		"@@f03 contexts\ncreate_callback\ncheck_position\ncallback_out\n" +
		"@@f03 others\n/etc/asterisk/ombutel/extensions__65-custom.conf\n" +
		"@@f03 taken\n799\n797\n" +
		"@@f03 waiting\n/f03callback/7599/1760550000.12          : 55551234\n1 results found.\n"
}

func TestParseCallbackState(t *testing.T) {
	file := CallbackDialplan(DefaultCallbackConfig(), "", []CallbackRoute{{"7599", "797"}})
	st, err := parseCallbackState(cannedState(file))
	if err != nil {
		t.Fatal(err)
	}
	if !st.HasCC || len(st.Queues) != 2 || st.Queues[1] != (CallbackQueue{ID: 9, Extension: "7599", Description: "PRUEBA CALLBACK", IVRID: 11, IVRName: "Callback", MemberGosub: "check_position",
		LiveContext: "IVR-11", LiveGosub: "check_position"}) {
		t.Fatalf("queues: %+v", st.Queues)
	}
	if st.Queues[0].LiveContext != "" || st.Queues[0].LiveGosub != "" || !st.live(st.Queues[1]) || st.live(st.Queues[0]) {
		t.Fatalf("rendered: 7599 is live, 7510 is not in the file: %+v", st.Queues)
	}
	if err := st.categoriesOK(); err != nil {
		t.Fatalf("categories: %v %v", st.Categories, err)
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
	if len(st.OtherFiles) != 1 || st.OtherFiles[0] != "/etc/asterisk/ombutel/extensions__65-custom.conf" {
		t.Fatalf("other files: %v", st.OtherFiles)
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
	for _, w := range []string{"|| echo '@@f03 error queues'", "| grep -v -x -F " + callbackFile + "; true", "awk '" + renderedAwk + "' " + queuesFile + " || echo '@@f03 error rendered'"} {
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
	for _, s := range []string{"for c in 799 797; do", "<<'SQL'", "dialplan show $c@cos-all-local", "database show f03callback"} {
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
		Categories:   map[string]string{"custom": "25", "custom_application": "4", "queue": "10", "queue_priority": "11", "terminate_call": "24"},
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
	st.Priorities, st.Apps = nil, st.Apps[:1]            // …and before the priority pieces
	st.TakenCodes = map[string]bool{"799": true}         // so 797 is free again and the file stays the same
	p, _ := PlanInstall(st, DefaultCallbackConfig(), []string{"7510"})
	if got := kinds(p); got != "file:omitir log:omitir app:omitir ivr:omitir priority:nuevo prioapp:nuevo queue:nuevo" {
		t.Fatalf("only the missing pieces: %s", got)
	}
}

func TestPlanInstallBlocksHandMadeDialplan(t *testing.T) {
	st := freshPBX()
	st.Contexts, st.OtherFiles = 3, []string{"/etc/asterisk/ombutel/extensions__65-custom.conf"}
	p, err := PlanInstall(st, DefaultCallbackConfig(), []string{"7510"})
	if err != nil {
		t.Fatal(err)
	}
	if s := p.Steps[0]; s.Kind != StepFile || s.Status != CbBlocked || !strings.Contains(s.Error, "extensions__65-custom.conf") {
		t.Fatalf("a hand-made callback dialplan must block the file step: %+v", s)
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
		"no CC": func() error {
			s := st
			s.HasCC = false
			_, err := PlanInstall(s, DefaultCallbackConfig(), []string{"7510"})
			return err
		},
		"no queues":     func() error { _, err := PlanInstall(st, DefaultCallbackConfig(), nil); return err },
		"unknown queue": func() error { _, err := PlanInstall(st, DefaultCallbackConfig(), []string{"9999"}); return err },
		"bad config": func() error {
			c := DefaultCallbackConfig()
			c.Prefix = "x"
			_, err := PlanInstall(st, c, []string{"7510"})
			return err
		},
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
	p, _ = PlanRemove(st, []string{"7510"}, false)
	if strings.Contains(kinds(p), "dropwaiting:nuevo") || p.Steps[3].Kind != StepDropWaiting || p.Steps[3].Status != CbSkip ||
		p.Steps[3].After != "se conservan: saldrán si vuelve a instalar (márquelo para descartarlas)" {
		t.Fatalf("waiting callbacks are only dropped when asked; otherwise the plan says they stay: %s", kinds(p))
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

func TestInstallCallbackFile(t *testing.T) {
	var cmd, body string
	addr, fp := fakeSSH(t, func(c string, in []byte) (string, string, int) { cmd, body = c, string(in); return "", "", 0 })
	p := sshProfile(t, addr, fp)
	ctx := context.Background()
	content := CallbackDialplan(DefaultCallbackConfig(), "", nil)
	if err := InstallCallbackFile(ctx, p, Secrets{SSH: "pw"}, content); err != nil {
		t.Fatal(err)
	}
	if body != content {
		t.Fatal("the file content travels on stdin, never in the command")
	}
	for _, s := range []string{"install -o asterisk -g asterisk -m 644", ".conf.f03tmp", "mv -f", "dialplan reload", "Context '$c'"} {
		if !strings.Contains(cmd, s) {
			t.Errorf("command lacks %q:\n%s", s, cmd)
		}
	}
	if strings.Contains(cmd, "[create_callback]") {
		t.Fatal("content leaked into the command line")
	}
}

func TestCallbackSSHFailuresCarryStderr(t *testing.T) {
	addr, fp := fakeSSH(t, func(string, []byte) (string, string, int) { return "", "falta el contexto check_position", 1 })
	p := sshProfile(t, addr, fp)
	if err := InstallCallbackFile(context.Background(), p, Secrets{SSH: "pw"}, "x"); err == nil || !strings.Contains(err.Error(), "check_position") {
		t.Fatalf("the remote reason must reach the user: %v", err)
	}
}

func TestEnsureLogAndDrop(t *testing.T) {
	var cmds []string
	addr, fp := fakeSSH(t, func(c string, _ []byte) (string, string, int) { cmds = append(cmds, c); return "", "", 0 })
	p := sshProfile(t, addr, fp)
	ctx := context.Background()
	if err := EnsureCallbackLog(ctx, p, Secrets{SSH: "pw"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmds[0], "[ -e "+callbackLog+" ] ||") || !strings.Contains(cmds[0], "chown asterisk:asterisk") {
		t.Fatalf("the log is created only when missing, never truncated: %s", cmds[0])
	}
	if err := DropCallbacks(ctx, p, Secrets{SSH: "pw"}, "7599"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmds[1], "show f03callback/7599") || !strings.Contains(cmds[1], "database deltree") {
		t.Fatalf("drop: %s", cmds[1])
	}
	if err := DropCallbacks(ctx, p, Secrets{SSH: "pw"}, "75;rm"); err == nil {
		t.Fatal("a non-digit queue must never reach the shell")
	}
}

func TestRouteCheck(t *testing.T) {
	reply := "[ Included context 'trk-group-1' created by 'pbx_config' ]\n  '_X.' =>          1. Noop(OR_TO_PBXU3CV15)\n\n-= 1 extension (1 priority) in 1 context. =-\n"
	var cmd string
	addr, fp := fakeSSH(t, func(c string, _ []byte) (string, string, int) { cmd = c; return reply, "", 0 })
	p := sshProfile(t, addr, fp)
	got, err := RouteCheck(context.Background(), p, Secrets{SSH: "pw"}, "955551234")
	if err != nil || got != "trk-group-1" || cmd != `asterisk -rx "dialplan show 955551234@ARS-all"` {
		t.Fatalf("got %q err %v cmd %q", got, err, cmd)
	}
	reply = "There is no existence of 955551234@ARS-all extension\n"
	if _, err := RouteCheck(context.Background(), p, Secrets{SSH: "pw"}, "955551234"); err == nil || !strings.Contains(err.Error(), "ninguna ruta de salida acepta el número marcado 955551234") {
		t.Fatalf("no route must block: %v", err)
	}
	if _, err := RouteCheck(context.Background(), p, Secrets{SSH: "pw"}, "1;id"); err == nil {
		t.Fatal("non-digits must never reach asterisk -rx")
	}
}

func TestQueueChangesOnlyAllowsF03Fields(t *testing.T) {
	before := map[string]string{"queue_id": "1", "strategy": "rrmemory", "ivr_id": "", "param:membergosub": ""}
	after := map[string]string{"queue_id": "1", "strategy": "rrmemory", "ivr_id": "11", "param:membergosub": "check_position"}
	if d := queueChanges(before, after); len(d) != 0 {
		t.Fatalf("IVR and membergosub are the only fields F-03 may change: %v", d)
	}
	after["strategy"], after["param:joinempty"] = "ringall", "no"
	d := queueChanges(before, after)
	if len(d) != 2 || !strings.Contains(strings.Join(d, ";"), "strategy: «rrmemory» → «ringall»") || !strings.Contains(strings.Join(d, ";"), "param:joinempty") {
		t.Fatalf("every other difference is reported, sorted: %v", d)
	}
}

// recPortal answers the login and records each module request's fields in order (multipart saves
// and urlencoded pbx_request calls alike). A getContent request answers editHTML as the form.
func recPortalHTML(t *testing.T, state, editHTML string) (*httptest.Server, *[][][2]string) {
	b, _ := json.Marshal(map[string]string{"state": state, "html": editHTML})
	return recPortalReplies(t, string(b))
}

// recPortalReplies answers module request n with replies[n] (the last one repeats).
func recPortalReplies(t *testing.T, replies ...string) (*httptest.Server, *[][][2]string) {
	var calls [][][2]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "x"})
			io.WriteString(w, `{"state":"success"}`)
			return
		}
		var fields [][2]string
		mt, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if mt == "multipart/form-data" {
			mr := multipart.NewReader(r.Body, params["boundary"])
			for {
				part, err := mr.NextPart()
				if err != nil {
					break
				}
				v, _ := io.ReadAll(part)
				fields = append(fields, [2]string{part.FormName(), string(v)})
			}
		} else {
			r.ParseForm()
			for _, k := range []string{"class", "method", "mode", "data", "baseurl"} {
				fields = append(fields, [2]string{k, r.PostForm.Get(k)})
			}
		}
		calls = append(calls, fields)
		io.WriteString(w, replies[min(len(calls), len(replies))-1])
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func recPortal(t *testing.T, state string) (*httptest.Server, *[][][2]string) {
	return recPortalHTML(t, state, "<form></form>")
}

func openFake(t *testing.T, srv *httptest.Server) *CallbackPortal {
	p := Profile{API: APIConfig{BaseURL: srv.URL, Enabled: true}}
	cp, err := OpenCallbackPortal(context.Background(), p, Secrets{API: "pw"})
	if err != nil {
		t.Fatal(err)
	}
	return cp
}

func TestPortalCallMultipartAndState(t *testing.T) {
	srv, calls := recPortal(t, "success")
	pt, _ := newPortal(Profile{API: APIConfig{BaseURL: srv.URL}})
	if err := pt.login(context.Background(), "admin", "pw"); err != nil {
		t.Fatal(err)
	}
	r, err := pt.call(context.Background(), [][2]string{{"class", "custom_app"}, {"method", "put"}})
	if err != nil || r.HTML != "<form></form>" {
		t.Fatalf("reply %+v err %v", r, err)
	}
	got := (*calls)[0]
	if got[0] != [2]string{"class", "custom_app"} || got[len(got)-1] != [2]string{"baseurl", srv.URL} {
		t.Fatalf("fields keep their order and end with baseurl: %v", got)
	}
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "x"})
			io.WriteString(w, `{"state":"success"}`)
			return
		}
		io.WriteString(w, `{"state":"exception","notification":{"text":"Código duplicado"}}`)
	}))
	t.Cleanup(srv2.Close)
	pt2, _ := newPortal(Profile{API: APIConfig{BaseURL: srv2.URL}})
	pt2.login(context.Background(), "admin", "pw")
	if _, err := pt2.call(context.Background(), [][2]string{{"class", "x"}}); err == nil || !strings.Contains(err.Error(), "Código duplicado") {
		t.Fatalf("a refused save carries the portal's text: %v", err)
	}
}

func TestPortalCallDialogIsFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "x"})
			io.WriteString(w, `{"state":"success"}`)
			return
		}
		io.WriteString(w, `{"state":"success","html":"<form></form>","dialog":"<div class=\"modal-module-errors\"><ul><li><p>Invalid destination is required and must be valid.</p><p>Opci&oacute;n no v&aacute;lida</p></li></ul></div>"}`)
	}))
	t.Cleanup(srv.Close)
	pt, _ := newPortal(Profile{API: APIConfig{BaseURL: srv.URL}})
	pt.login(context.Background(), "admin", "pw")
	if _, err := pt.call(context.Background(), [][2]string{{"class", "x"}}); err == nil || !strings.Contains(err.Error(), "Invalid destination is required") || !strings.Contains(err.Error(), "Opción no válida") {
		t.Fatalf("a dialog is a failure: %v", err)
	}
	if _, err := pt.request(context.Background(), "x", "delete", "delete", "1"); err == nil {
		t.Fatal("request applies the same dialog check")
	}
}

const sampleForm = `<form id="f"><input name="a" value="1"><input type="checkbox" name="c1" value="y"><input type="checkbox" name="c2" value="y" checked>
<select name="s1"><option value="o1">One</option><option value="o2" selected>Two</option></select>
<select name="s2"><option value="p1">One</option><option value="p2">Two</option></select>
<textarea name="t">hello</textarea><input name="d" value="no" disabled><input type="submit" name="go" value="x">
<input name="x[{{row-count-placeholder}}][a]" value=""><input type="checkbox" name="c3"><input type="checkbox" name="c4" checked>
<select name="m[]" multiple><option value="1" selected>1</option><option value="2">2</option><option value="3" selected>3</option></select>
<select name="m0[]" multiple><option value="1">1</option></select>
<select name="last"><option value="1" selected>1</option><option value="2" selected>2</option></select>
<select name="dis"><option value="1" disabled>1</option><option value="2">2</option></select>
<select name="alldis"><option value="1" disabled>1</option></select></form>`

func TestFormFields(t *testing.T) {
	got, err := formFields(sampleForm, "f")
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]string{{"a", "1"}, {"c2", "y"}, {"s1", "o2"}, {"s2", "p1"}, {"t", "hello"}, {"x[{{row-count-placeholder}}][a]", ""}, {"c4", "on"},
		{"m[]", "1"}, {"m[]", "3"}, {"last", "2"}, {"dis", "2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if _, err := formFields(sampleForm, "nope"); err == nil {
		t.Fatal("a missing form is an error")
	}
}

func lastCall(t *testing.T, calls *[][][2]string) [][2]string {
	if len(*calls) == 0 {
		t.Fatal("the portal got no request")
	}
	return (*calls)[len(*calls)-1]
}

func wantFields(t *testing.T, got, want [][2]string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fields differ\n got %v\nwant %v", got, want)
	}
}

func TestPortalCreateCallbackApp(t *testing.T) {
	srv, calls := recPortal(t, "success")
	cp := openFake(t, srv)
	if err := cp.createCallbackApp(context.Background(), "797"); err != nil {
		t.Fatal(err)
	}
	wantFields(t, lastCall(t, calls), [][2]string{{"extension", "797"}, {"description", "Callback"}, {"status", "yes"}, {"direct", "yes"},
		{"mod_dest", "25"}, {"destination_index", ""}, {"custom_destination", "create_callback,s,1"},
		{"mod_dest_return", "24"}, {"return_destination_index", "1"}, {"custom_category", "25"},
		{"class", "custom_app"}, {"method", "put"}, {"mode", "add"}, {"baseurl", srv.URL}})
}

func TestPortalCreateCallbackIVR(t *testing.T) {
	srv, calls := recPortal(t, "success")
	cp := openFake(t, srv)
	if err := cp.createCallbackIVR(context.Background(), "9", CallbackApp{ID: 42}); err != nil {
		t.Fatal(err)
	}
	wantFields(t, lastCall(t, calls), [][2]string{{"description", "Callback"}, {"class_of_service_id", ""}, {"ivr_current_cos", ""},
		{"invalid_tries", "3"}, {"welcome_msg_id", ""}, {"instructions_msg_id", ""}, {"invalid_retry_msg_id", ""}, {"invalid_msg_id", ""},
		{"response_timeout", "10"}, {"digit_timeout", "2"}, {"timeout_tries", "3"}, {"timeout_retry_msg_id", ""}, {"timeout_msg_id", ""},
		{"timeout_add_msg", "yes"}, {"invalid_add_msg", "yes"}, {"mod_invalid", "24"}, {"invalid_index", "1"}, {"mod_timeout", "24"}, {"timeout_index", "1"},
		{"entries[{{row-count-placeholder}}][option]", ""}, {"entries[{{row-count-placeholder}}][mod_dest]", ""},
		{"entries[{{row-count-placeholder}}][dest]", ""}, {"entries[{{row-count-placeholder}}][enabled]", "1"},
		{"entries[0][option]", "9"}, {"entries[0][mod_dest]", "4"}, {"entries[0][dest]", "42"}, {"entries[0][enabled]", "1"},
		{"class", "ivr"}, {"method", "put"}, {"mode", "add"}, {"baseurl", srv.URL}})
}

func TestPortalCreatePriority(t *testing.T) {
	srv, calls := recPortal(t, "success")
	cp := openFake(t, srv)
	if err := cp.createPriority(context.Background(), CallbackQueue{ID: 5, Extension: "7510"}, 10); err != nil {
		t.Fatal(err)
	}
	wantFields(t, lastCall(t, calls), [][2]string{{"description", "Callback 7510"}, {"priority", "10"}, {"mod_dest", "10"}, {"destination_index", "5"},
		{"class", "queues_priority"}, {"method", "put"}, {"mode", "add"}, {"baseurl", srv.URL}})
}

func TestPortalCreatePrioApp(t *testing.T) {
	srv, calls := recPortal(t, "success")
	cp := openFake(t, srv)
	if err := cp.createPrioApp(context.Background(), "796", CallbackQueue{ID: 5, Extension: "7510"}, CallbackPriority{ID: 8}); err != nil {
		t.Fatal(err)
	}
	wantFields(t, lastCall(t, calls), [][2]string{{"extension", "796"}, {"description", "Callback 7510"}, {"status", "yes"}, {"direct", "yes"},
		{"mod_dest", "11"}, {"destination_index", "8"}, {"custom_destination", ""},
		{"mod_dest_return", "24"}, {"return_destination_index", "1"}, {"custom_category", "25"},
		{"class", "custom_app"}, {"method", "put"}, {"mode", "add"}, {"baseurl", srv.URL}})
}

const queueForm = `<form id="queues_form"><input type="hidden" name="class" value="queues"><input type="hidden" name="method" value="put"><input type="hidden" name="mode" value="edit">
<input name="tab-content" value="5"><input name="extension" value="7510">
<select name="strategy"><option value="ringall">r</option><option value="rrmemory" selected>m</option></select>
<select name="ivr_id"><option value="" selected>none</option><option value="11">Callback</option></select>
<input type="checkbox" name="autofill" value="1" checked>
<input name="advanced_name[]" value="joinempty"><input name="advanced_value[]" value="no">
<input name="advanced_name[]" value="membergosub"><input name="advanced_value[]" value="check_position">
<input name="tab-content" value="5"></form>`

func TestPortalAttachQueueKeepsForm(t *testing.T) {
	srv, calls := recPortalHTML(t, "success", queueForm)
	cp := openFake(t, srv)
	q := CallbackQueue{ID: 5, Extension: "7510"}
	if err := cp.setQueue(context.Background(), q, 11, "check_position"); err != nil {
		t.Fatal(err)
	}
	wantFields(t, (*calls)[0], [][2]string{{"class", "queues"}, {"method", "getContent"}, {"mode", "edit"}, {"data", "5"}, {"baseurl", srv.URL}})
	wantFields(t, (*calls)[1], [][2]string{{"class", "queues"}, {"method", "put"}, {"mode", "edit"},
		{"tab-content", "5"}, {"extension", "7510"}, {"strategy", "rrmemory"}, {"ivr_id", "11"}, {"autofill", "1"},
		{"advanced_name[]", "joinempty"}, {"advanced_value[]", "no"}, {"advanced_name[]", "membergosub"}, {"advanced_value[]", "check_position"},
		{"tab-content", "5"}, {"queue_id", "5"}, {"baseurl", srv.URL}})
	// detach: IVR cleared, membergosub row removed, everything else unchanged
	if err := cp.setQueue(context.Background(), q, 0, ""); err != nil {
		t.Fatal(err)
	}
	wantFields(t, lastCall(t, calls), [][2]string{{"class", "queues"}, {"method", "put"}, {"mode", "edit"},
		{"tab-content", "5"}, {"extension", "7510"}, {"strategy", "rrmemory"}, {"ivr_id", ""}, {"autofill", "1"},
		{"advanced_name[]", "joinempty"}, {"advanced_value[]", "no"},
		{"tab-content", "5"}, {"queue_id", "5"}, {"baseurl", srv.URL}})
}

func TestPortalAttachQueueAddsGosubAfterExisting(t *testing.T) {
	form := `<form id="queues_form"><input name="extension" value="7510"><select name="ivr_id"><option value="">-</option></select><input name="advanced_name[]" value="joinempty"><input name="advanced_value[]" value="no"><input name="queue_id" value="5"></form>`
	srv, calls := recPortalHTML(t, "success", form)
	cp := openFake(t, srv)
	if err := cp.setQueue(context.Background(), CallbackQueue{ID: 5, Extension: "7510"}, 11, "check_position"); err != nil {
		t.Fatal(err)
	}
	wantFields(t, lastCall(t, calls), [][2]string{{"extension", "7510"}, {"ivr_id", "11"}, {"advanced_name[]", "joinempty"}, {"advanced_value[]", "no"},
		{"advanced_name[]", "membergosub"}, {"advanced_value[]", "check_position"}, {"queue_id", "5"},
		{"class", "queues"}, {"method", "put"}, {"mode", "edit"}, {"baseurl", srv.URL}})
}

func TestPortalSetQueueRefusesForeignForm(t *testing.T) {
	for name, form := range map[string]string{
		"other extension": `<form id="queues_form"><input name="extension" value="9999"><input name="ivr_id" value=""></form>`,
		"no ivr_id":       `<form id="queues_form"><input name="extension" value="7510"></form>`,
		"other queue_id":  `<form id="queues_form"><input name="extension" value="7510"><input name="ivr_id" value=""><input name="queue_id" value="6"></form>`,
	} {
		srv, calls := recPortalHTML(t, "success", form)
		cp := openFake(t, srv)
		err := cp.setQueue(context.Background(), CallbackQueue{ID: 5, Extension: "7510"}, 11, "check_position")
		if err == nil || !strings.Contains(err.Error(), "no corresponde a la cola 7510") {
			t.Fatalf("%s: want a refusal, got %v", name, err)
		}
		if len(*calls) != 1 {
			t.Fatalf("%s: only the getContent may be sent, got %d requests", name, len(*calls))
		}
	}
}

func TestSetGosubKeepsForeign(t *testing.T) {
	f := [][2]string{{"advanced_name[]", "membergosub"}, {"advanced_value[]", "other_sub"}}
	wantFields(t, setGosub(f, ""), [][2]string{{"advanced_name[]", "membergosub"}, {"advanced_value[]", "other_sub"}})
	f = [][2]string{{"advanced_name[]", "membergosub"}, {"advanced_value[]", "check_position"}}
	if got := setGosub(f, ""); len(got) != 0 {
		t.Fatalf("F-03's own row is removed: %v", got)
	}
}

func TestDetachIVR(t *testing.T) {
	st := CallbackState{
		Apps: []CallbackApp{{ID: 1, DestKind: "custom", Custom: callbackTarget}},
		IVRs: []CallbackIVR{{ID: 11, Entries: []CallbackIVREntry{{Option: "9", DestKind: "custom_application", DestIndex: 1}}}},
	}
	if got := detachIVR(st, CallbackQueue{IVRID: 30}); got != 30 {
		t.Fatalf("foreign IVR is kept: %d", got)
	}
	if got := detachIVR(st, CallbackQueue{IVRID: 11}); got != 0 {
		t.Fatalf("callback IVR goes: %d", got)
	}
	if got := detachIVR(st, CallbackQueue{}); got != 0 {
		t.Fatalf("no IVR: %d", got)
	}
	if got := detachIVR(CallbackState{}, CallbackQueue{IVRID: 30}); got != 30 {
		t.Fatalf("no callback IVR at all: foreign kept: %d", got)
	}
}

func TestPortalDeletes(t *testing.T) {
	srv, calls := recPortal(t, "success")
	cp := openFake(t, srv)
	ctx := context.Background()
	for _, c := range []struct {
		do    func(context.Context, int) error
		class string
	}{{cp.deleteApp, "custom_app"}, {cp.deleteIVR, "ivr"}, {cp.deletePriority, "queues_priority"}} {
		if err := c.do(ctx, 7); err != nil {
			t.Fatal(err)
		}
		wantFields(t, lastCall(t, calls), [][2]string{{"class", c.class}, {"method", "delete"}, {"mode", "delete"}, {"data", "7"}, {"baseurl", srv.URL}})
	}
}

func deleteModal(t *testing.T, id string) string {
	h := `<div id="modal-module-delete"><form method="POST" action="/" id="modal-form-delete"><p>Are you sure?</p>` +
		`<input type="hidden" value="7" name="custom_application_id" id="custom_application_id">` +
		`<input type="hidden" value="custom_app" name="class"><input type="hidden" value="delete" name="method">` +
		`<input type="hidden" value="deleteConfirmed" name="mode"><input type="hidden" value="` + id + `" name="data">` +
		`<button type="button">Cancel</button><button type="submit">Delete</button></form></div>`
	b, _ := json.Marshal(map[string]string{"state": "success", "dialog": "modal-module-delete", "html": h})
	return string(b)
}

func TestPortalDeleteConfirms(t *testing.T) {
	srv, calls := recPortalReplies(t, deleteModal(t, "7"), `{"state":"success","notification":{"text":"The data has been removed"}}`)
	cp := openFake(t, srv)
	if err := cp.deleteApp(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 2 {
		t.Fatalf("want 2 requests, got %d", len(*calls))
	}
	wantFields(t, (*calls)[0], [][2]string{{"class", "custom_app"}, {"method", "delete"}, {"mode", "delete"}, {"data", "7"}, {"baseurl", srv.URL}})
	wantFields(t, (*calls)[1], [][2]string{{"custom_application_id", "7"}, {"class", "custom_app"}, {"method", "delete"},
		{"mode", "deleteConfirmed"}, {"data", "7"}, {"baseurl", srv.URL}})
}

func TestPortalDeleteRefusesUnexpectedConfirm(t *testing.T) {
	srv, calls := recPortalReplies(t, deleteModal(t, "8"))
	err := openFake(t, srv).deleteApp(context.Background(), 7)
	if err == nil || len(*calls) != 1 {
		t.Fatalf("a confirmation for another id must be refused without posting: %v, %d requests", err, len(*calls))
	}
}

func TestPortalDeleteInUse(t *testing.T) {
	srv, calls := recPortalReplies(t, `{"state":"success","dialog":"modal-module-errors","html":"<p>Cannot remove this object, it is used as destination by X</p>"}`,
		`{"state":"success","dialog":"<p>Cannot remove this object, it is used as destination by X</p>"}`)
	err := openFake(t, srv).deleteApp(context.Background(), 7)
	if err == nil || !strings.Contains(err.Error(), "Cannot remove this object") || strings.Contains(err.Error(), "modal-module") || len(*calls) != 1 {
		t.Fatalf("in use must fail with the portal's text only and one request: %v, %d", err, len(*calls))
	}
}

func TestPortalDeleteRefusesDuplicateField(t *testing.T) {
	var r map[string]string
	json.Unmarshal([]byte(deleteModal(t, "7")), &r)
	r["html"] = strings.Replace(r["html"], `<button`, `<input type="hidden" value="8" name="data"><button`, 1)
	b, _ := json.Marshal(r)
	srv, calls := recPortalReplies(t, string(b))
	if err := openFake(t, srv).deleteApp(context.Background(), 7); err == nil || len(*calls) != 1 {
		t.Fatalf("a duplicate data field must be refused without posting: %v, %d", err, len(*calls))
	}
}

func TestCheckReplyCapsDialogText(t *testing.T) {
	err := checkReply(portalReply{State: "success", Dialog: json.RawMessage(`"modal-module-errors"`), HTML: "<p>" + strings.Repeat("x", 1000) + "</p>"})
	if err == nil || strings.Contains(err.Error(), "modal-module") || len([]rune(err.Error())) > 400 {
		t.Fatalf("want capped text without the modal name: %d runes, %v", len([]rune(err.Error())), err)
	}
}

// twoQueues: installed() plus 75102 attached too (priority 4, app 796); the file has both routes.
func twoQueues() CallbackState {
	st := installed()
	st.Queues[1].IVRID, st.Queues[1].IVRName, st.Queues[1].MemberGosub = 11, "Callback", "check_position"
	st.Priorities = append(st.Priorities, CallbackPriority{ID: 4, Description: "Callback 75102", Priority: 10, DestKind: "queue", DestIndex: 2})
	st.Apps = append(st.Apps, CallbackApp{ID: 7, Extension: "796", Description: "Callback 75102", DestKind: "queue_priority", DestIndex: 4})
	st.File = CallbackDialplan(DefaultCallbackConfig(), "", []CallbackRoute{{"7510", "797"}, {"75102", "796"}})
	return st
}

func TestPlanRemoveWaitsWhileQueueIsLive(t *testing.T) {
	st := installed()
	st.Queues[0].LiveContext, st.Queues[0].LiveGosub = "IVR-11", "check_position"
	p, err := PlanRemove(st, []string{"7510"}, false)
	if err != nil || kinds(p) != "detach:nuevo delprioapp:nuevo delpriority:nuevo delivr:nuevo delapp:nuevo delfile:omitir" {
		t.Fatalf("the portal pieces go now, the file waits: %s %v", kinds(p), err)
	}
	if f := p.Steps[len(p.Steps)-1]; f.After != fileWaitText || !p.FileWaits || p.Dialplan != "" {
		t.Fatalf("the waiting file step says why: %+v", p)
	}
	got := RunCallback(context.Background(), p, func(context.Context, CallbackStep) error { return nil }, func(int) {})
	if f := got[len(got)-1]; f.Status != CbSkipped || f.Error != fileWaitText {
		t.Fatalf("the result says the file waits, not that it was already so: %+v", f)
	}
	for name, mod := range map[string]func(*CallbackQueue){
		"only the IVR context": func(q *CallbackQueue) { q.LiveGosub = "" },
		"only the membergosub": func(q *CallbackQueue) { q.LiveContext = "" },
	} {
		s := installed()
		s.Queues[0].LiveContext, s.Queues[0].LiveGosub = "IVR-11", "check_position"
		mod(&s.Queues[0])
		if p, _ := PlanRemove(s, []string{"7510"}, false); !p.FileWaits {
			t.Errorf("%s still runs F-03: %s", name, kinds(p))
		}
	}
	st = installed()
	st.Queues[0].LiveContext = "IVR-2" // a foreign IVR rendered: not F-03's
	if p, err = PlanRemove(st, []string{"7510"}, false); err != nil || p.FileWaits || kinds(p) != "detach:nuevo delprioapp:nuevo delpriority:nuevo delivr:nuevo delapp:nuevo delfile:nuevo" {
		t.Fatalf("not live: the file goes now: %s %v", kinds(p), err)
	}
	st = installed() // the callback IVR is already gone from the portal; Asterisk still runs its context
	st.Queues[0].IVRID, st.Queues[0].IVRName, st.Queues[0].MemberGosub, st.IVRs = 0, "", "", nil
	st.Queues[0].LiveContext = "IVR-11"
	if !st.live(st.Queues[0]) {
		t.Fatal("a rendered IVR the portal no longer has for the queue may be the old callback IVR: live")
	}
}

func TestPlanRemoveKeepsRoutesOfLiveQueues(t *testing.T) {
	st := twoQueues()
	st.Queues[0].LiveContext, st.Queues[0].LiveGosub = "IVR-11", "check_position"
	st.Queues[1].LiveContext, st.Queues[1].LiveGosub = "IVR-11", "check_position"
	p, err := PlanRemove(st, []string{"7510"}, false)
	if err != nil || len(p.Routes) != 2 || p.Routes[0] != (CallbackRoute{"7510", "797"}) || p.Routes[1] != (CallbackRoute{"75102", "796"}) {
		t.Fatalf("7510 is still live: its callbacks keep their route: %+v %v", p.Routes, err)
	}
	if kinds(p) != "detach:nuevo delprioapp:nuevo delpriority:nuevo" {
		t.Fatalf("the file already has both routes: nothing to rewrite: %s", kinds(p))
	}
	st.Contexts = 2 // a rewrite is needed, but 7510 is live: it waits
	if p, _ = PlanRemove(st, []string{"7510"}, false); kinds(p) != "detach:nuevo delprioapp:nuevo delpriority:nuevo file:omitir" || !p.FileWaits {
		t.Fatalf("the rewrite waits too: %s", kinds(p))
	}
	st = twoQueues()
	st.Queues[0].LiveGosub = "check_position" // detached in the portal earlier, not applied yet
	st.Queues[0].IVRID, st.Queues[0].IVRName, st.Queues[0].MemberGosub = 0, "", ""
	ip, err := PlanInstall(st, DefaultCallbackConfig(), []string{"75102"})
	if err != nil || len(ip.Routes) != 2 {
		t.Fatalf("an install keeps the route of a queue that is still live: %+v %v", ip.Routes, err)
	}
}

func TestPlanRemoveSharedLeftovers(t *testing.T) {
	st := freshPBX() // a stopped install: file, log and app only
	st.Apps = []CallbackApp{{ID: 5, Extension: "799", Description: "Callback", DestKind: "custom", Custom: callbackTarget}}
	st.File, st.FileExists, st.LogOwner, st.Contexts = CallbackDialplan(DefaultCallbackConfig(), "", nil), true, "asterisk", 3
	p, err := PlanRemove(st, nil, false)
	if err != nil || kinds(p) != "delapp:nuevo delfile:nuevo" {
		t.Fatalf("no queues: only the shared pieces: %s %v", kinds(p), err)
	}
	if _, err := PlanRemove(installed(), nil, false); err == nil || err.Error() != "elija las colas que tienen devolución de llamada" {
		t.Fatalf("a hooked queue must be chosen: %v", err)
	}
	st.Queues[0].LiveGosub = "check_position"
	if _, err := PlanRemove(st, nil, false); err == nil || !strings.Contains(err.Error(), "Aplicar cambios en la PBX") {
		t.Fatalf("a queue still live: apply first: %v", err)
	}
	if _, err := PlanRemove(freshPBX(), nil, false); err == nil {
		t.Fatal("nothing to remove must be said, not planned empty")
	}
}

func TestQueueViewsRemovableAndLive(t *testing.T) {
	st := freshPBX()
	st.Priorities = []CallbackPriority{{ID: 3, Description: "Callback 7510", Priority: 10, DestKind: "queue", DestIndex: 1}}
	st.Apps = []CallbackApp{{ID: 6, Extension: "797", Description: "Callback 7510", DestKind: "queue_priority", DestIndex: 3}}
	st.Queues[1].LiveGosub = "check_position"
	v := st.QueueViews()
	if !v[0].Removable || v[0].Hooked || v[0].PriorityApp != "797" || v[0].Live {
		t.Fatalf("7510 has only its priority pieces: removable: %+v", v[0])
	}
	if v[1].Removable || !v[1].Live || v[1].PriorityApp != "" {
		t.Fatalf("75102: nothing in the portal, still live: %+v", v[1])
	}
	if _, err := PlanRemove(st, []string{"7510"}, false); err != nil {
		t.Fatalf("Removable matches PlanRemove: %v", err)
	}
}

func TestPlansRefuseUnknownCategories(t *testing.T) {
	for name, mod := range map[string]func(map[string]string){
		"other id": func(c map[string]string) { c["queue"] = "12" },
		"missing":  func(c map[string]string) { delete(c, "terminate_call") },
	} {
		st := installed()
		mod(st.Categories)
		_, ei := PlanInstall(st, DefaultCallbackConfig(), []string{"75102"})
		_, er := PlanRemove(st, []string{"7510"}, false)
		for _, err := range []error{ei, er} {
			if err == nil || !strings.Contains(err.Error(), "categorías de destino") {
				t.Errorf("%s: nothing may be written: %v", name, err)
			}
		}
	}
}

func TestPlanInstallKeepsInstalledKeyAndPriority(t *testing.T) {
	c := DefaultCallbackConfig()
	c.Key, c.Priority = "5", 20
	p, err := PlanInstall(installed(), c, []string{"7510"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Steps[3].Kind != StepIVR || p.Steps[3].After != "ya instalado con la tecla 0: se mantiene 0 (quite e instale de nuevo para cambiarla)" || p.Steps[3].Code != "0" {
		t.Fatalf("the IVR keeps its key and says so: %+v", p.Steps[3])
	}
	if got, ok := ParseCallbackConfig(p.Dialplan); p.Config.Key != "0" || !ok || got.Key != "0" {
		t.Fatalf("the file header records the installed key: %+v %+v", p.Config, got)
	}
	if p.Steps[4].Kind != StepPriority || !strings.Contains(p.Steps[4].After, "prioridad 10: se mantiene 10") {
		t.Fatalf("the priority keeps its value and says so: %+v", p.Steps[4])
	}
}

func TestStepRunnerQueueRechecksForeign(t *testing.T) {
	state := strings.Replace(cannedState(""), "1\t7510\tCola CC\t0\t\t\n", "1\t7510\tCola CC\t0\t\totra_cosa\n", 1)
	addr, fp := fakeSSH(t, func(string, []byte) (string, string, int) { return state, "", 0 })
	srv, calls := recPortal(t, "success")
	cp := openFake(t, srv)
	run := CallbackStepRunner(sshProfile(t, addr, fp), Secrets{SSH: "pw"}, cp, CallbackPlan{})
	if err := run(context.Background(), CallbackStep{Kind: StepQueue, Queue: "7510"}); err == nil || !strings.Contains(err.Error(), "otra_cosa") {
		t.Fatalf("a membergosub set since the preview stops the step: %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("nothing may reach the portal: %v", *calls)
	}
}

func TestStepRunnerIVRUsesAppIDFromState(t *testing.T) {
	addr, fp := fakeSSH(t, func(string, []byte) (string, string, int) { return cannedState(""), "", 0 })
	srv, calls := recPortal(t, "success")
	cp := openFake(t, srv)
	plan := CallbackPlan{Config: DefaultCallbackConfig()}
	run := CallbackStepRunner(sshProfile(t, addr, fp), Secrets{SSH: "pw"}, cp, plan)
	for _, s := range []CallbackStep{{Kind: StepApp, Code: "799"}, {Kind: StepIVR, Code: "0"}} {
		if err := run(context.Background(), s); err != nil {
			t.Fatal(err)
		}
	}
	if len(*calls) != 2 || (*calls)[0][0] != [2]string{"extension", "799"} {
		t.Fatalf("app then IVR: %v", *calls)
	}
	ivr := (*calls)[1]
	if !reflect.DeepEqual(ivr[23:27], [][2]string{{"entries[0][option]", "0"}, {"entries[0][mod_dest]", "4"}, {"entries[0][dest]", "5"}, {"entries[0][enabled]", "1"}}) {
		t.Fatalf("the IVR entry goes to app id 5 from the state read: %v", ivr)
	}
}

// snapPBX answers the queue snapshot: the table lookup, then the bash -s script with *members as the
// ombu_queue_members rows. fail makes every SSH call fail.
func snapPBX(t *testing.T, members *string, fail *bool, script *string) Profile {
	addr, fp := fakeSSH(t, func(cmd string, in []byte) (string, string, int) {
		if *fail {
			return "", "sin conexión", 1
		}
		if cmd == mysqlCmd {
			return "t\tpk\nombu_queue_departments\tqueue_id,department_id\nombu_queue_members\tqueue_member_id\n", "", 0
		}
		*script = string(in)
		return "@@f03 queue\nqueue_id\textension\tstrategy\n5\t7510\trrmemory\n@@f03 params\nname\tvalue\njoinempty\tno\n" +
			"@@f03 table ombu_queue_departments\nqueue_id\tdepartment_id\n5\t3\n" +
			"@@f03 table ombu_queue_members\n" + *members, "", 0
	})
	return sshProfile(t, addr, fp)
}

func TestGuardQueueCoversQueueTables(t *testing.T) {
	members := "queue_member_id\tqueue_id\tinterface\tpenalty\n1\t5\tSIP/2000\t0\n2\t5\tSIP/2001\t0\n"
	fail := false
	var script string
	cp := &CallbackPortal{p: snapPBX(t, &members, &fail, &script), sec: Secrets{SSH: "pw"}}
	q := CallbackQueue{ID: 5, Extension: "7510"}
	ctx := context.Background()
	err := cp.guardQueue(ctx, q, func() error {
		members = "queue_member_id\tqueue_id\tinterface\tpenalty\n1\t5\tSIP/2000\t3\n2\t5\tSIP/2001\t0\n"
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "ombu_queue_members") {
		t.Fatalf("a changed agent penalty must be reported: %v", err)
	}
	if !strings.Contains(script, "SELECT * FROM `ombu_queue_members` WHERE queue_id = 5;") || !strings.Contains(script, "ombu_queue_params WHERE queue_id = 5") {
		t.Fatalf("one script reads every table: %s", script)
	}
	err = cp.guardQueue(ctx, q, func() error { // re-saved: same agents, other order, renumbered keys
		members = "queue_member_id\tqueue_id\tinterface\tpenalty\n8\t5\tSIP/2001\t0\n9\t5\tSIP/2000\t3\n"
		return nil
	})
	if err != nil {
		t.Fatalf("the same rows in another order are no change: %v", err)
	}
	err = cp.guardQueue(ctx, q, func() error { fail = true; return errors.New("el portal rechazó el cambio: X") })
	if err == nil || !strings.Contains(err.Error(), "rechazó el cambio: X; además, no se pudo verificar la cola 7510 después de guardar") {
		t.Fatalf("both failures are reported: %v", err)
	}
}

func TestPortalDialogFalseIsNoFailure(t *testing.T) {
	for body, fails := range map[string]bool{
		`{"state":"success","dialog":false}`:              false,
		`{"state":"success","dialog":null}`:               false,
		`{"state":"success","dialog":""}`:                 false,
		`{"state":"success","dialog":{"errors":["mal"]}}`: true,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/login" {
				http.SetCookie(w, &http.Cookie{Name: "sid", Value: "x"})
				io.WriteString(w, `{"state":"success"}`)
				return
			}
			io.WriteString(w, body)
		}))
		pt, _ := newPortal(Profile{API: APIConfig{BaseURL: srv.URL}})
		pt.login(context.Background(), "admin", "pw")
		_, err := pt.call(context.Background(), [][2]string{{"class", "x"}})
		srv.Close()
		if (err != nil) != fails || fails && !strings.Contains(err.Error(), "mal") {
			t.Errorf("%s: want failure %v, got %v", body, fails, err)
		}
	}
}
