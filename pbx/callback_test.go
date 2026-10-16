package pbx

import (
	"context"
	"encoding/base64"
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
