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
		"no preview":  (*pinPlan)(nil).check("A", 7),
		"other PBX":   plan.check("B", 7),
		"other list":  plan.check("A", 8),
		"error rows":  (&pinPlan{profileID: "A", listID: 7, rows: []pbx.PinRow{{Status: pbx.PinNew}, {Status: pbx.PinError}}}).check("A", 7),
		"nothing new": (&pinPlan{profileID: "A", listID: 7, rows: []pbx.PinRow{{Status: pbx.PinExists}, {Status: pbx.PinNoDesc}}}).check("A", 7),
	} {
		if err == nil {
			t.Errorf("%s: Apply must be refused", name)
		}
	}
	if err := plan.check("B", 7); !strings.Contains(err.Error(), "no corresponde") {
		t.Errorf("message must say the preview belongs elsewhere: %v", err)
	}
}
