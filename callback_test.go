package main

import (
	"strings"
	"testing"

	"github.com/skemono/Xorcom-Tools/pbx"
)

func cbPlan(statuses ...string) pbx.CallbackPlan {
	var p pbx.CallbackPlan
	for i, s := range statuses {
		p.Steps = append(p.Steps, pbx.CallbackStep{Kind: pbx.StepApp, Code: string(rune('0' + i)), Status: s})
	}
	return p
}

func TestCallbackRunCheck(t *testing.T) {
	ok := &callbackRun{profileID: "A", plan: cbPlan(pbx.CbNew, pbx.CbSkip)}
	if err := ok.check("A"); err != nil {
		t.Fatalf("a plan with work must pass: %v", err)
	}
	for name, err := range map[string]error{
		"no preview":   (*callbackRun)(nil).check("A"),
		"other PBX":    ok.check("B"),
		"blocked step": (&callbackRun{profileID: "A", plan: cbPlan(pbx.CbNew, pbx.CbBlocked)}).check("A"),
		"nothing new":  (&callbackRun{profileID: "A", plan: cbPlan(pbx.CbSkip)}).check("A"),
	} {
		if err == nil {
			t.Errorf("%s: Aplicar must be refused", name)
		}
	}
}

func TestCallbackApplyRefusesChangedPBX(t *testing.T) {
	a := cbPlan(pbx.CbNew, pbx.CbSkip)
	if !sameSteps(a, cbPlan(pbx.CbNew, pbx.CbSkip)) {
		t.Fatal("an identical re-plan matches")
	}
	b := cbPlan(pbx.CbSkip, pbx.CbSkip) // someone created the app in the portal meanwhile
	if sameSteps(a, b) {
		t.Fatal("a different re-plan must not match")
	}
	c := cbPlan(pbx.CbNew, pbx.CbSkip)
	c.Steps[0].Before = "otra cosa" // texts may change (a waiting count); kinds, codes and statuses may not
	if !sameSteps(a, c) {
		t.Fatal("only kind, queue, code and status are compared")
	}
	if err := errChanged; !strings.Contains(err.Error(), "cambió desde la vista previa") {
		t.Fatal(err)
	}
}
