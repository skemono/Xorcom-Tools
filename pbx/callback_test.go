package pbx

import (
	"os"
	"strings"
	"testing"
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
