package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/skemono/Xorcom-Tools/pbx"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// CallbackService is F-03: queue callback (Xorcom article) installed and removed on the active PBX.
type CallbackService struct {
	profiles *ProfileService
	mu       sync.Mutex
	last     *callbackRun // exactly what Apply runs: the last preview
}

type callbackRun struct {
	profileID string
	plan      pbx.CallbackPlan
}

// CallbackView is the Estado: the PBX state and the settings installed there (or the defaults).
type CallbackView struct {
	State     pbx.CallbackState       `json:"state"`
	Queues    []pbx.CallbackQueueView `json:"queues"`
	Config    pbx.CallbackConfig      `json:"config"`
	Installed bool                    `json:"installed"`
}

type CallbackPreview struct {
	Plan    pbx.CallbackPlan `json:"plan"`
	Route   string           `json:"route"` // install: the trunk group that accepts the test number
	New     int              `json:"new"`
	Skip    int              `json:"skip"`
	Blocked int              `json:"blocked"`
}

type CallbackResult struct {
	Folio   int                `json:"folio"`
	Steps   []pbx.CallbackStep `json:"steps"`
	Applied int                `json:"applied"`
	Skipped int                `json:"skipped"`
	Failed  int                `json:"failed"`
	NotRun  int                `json:"notRun"`
}

const (
	callbackReadTimeout  = 60 * time.Second
	callbackApplyTimeout = 5 * time.Minute
)

var errChanged = errors.New("la PBX cambió desde la vista previa: vuelva a generarla")

func (s *CallbackService) State() (CallbackView, error) {
	p, sec, err := s.profiles.active()
	if err != nil {
		return CallbackView{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), callbackReadTimeout)
	defer cancel()
	st, err := pbx.ReadCallbackState(ctx, p, sec, pbx.CandidateCodes(pbx.DefaultCallbackConfig().AppCode))
	if err != nil {
		return CallbackView{}, errors.New(pbx.Describe(err))
	}
	c, ok := pbx.ParseCallbackConfig(st.File)
	if !ok {
		c = pbx.DefaultCallbackConfig()
	}
	return CallbackView{State: st, Queues: st.QueueViews(), Config: c, Installed: ok}, nil
}

func (s *CallbackService) PreviewInstall(c pbx.CallbackConfig, queues []string, testNumber string) (CallbackPreview, error) {
	p, sec, err := s.profiles.active()
	if err != nil {
		return CallbackPreview{}, err
	}
	if err := c.Validate(); err != nil {
		return CallbackPreview{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), callbackReadTimeout)
	defer cancel()
	route, err := pbx.RouteCheck(ctx, p, sec, c.Prefix+testNumber)
	if err != nil {
		return CallbackPreview{}, errors.New(pbx.Describe(err))
	}
	st, err := pbx.ReadCallbackState(ctx, p, sec, pbx.CandidateCodes(c.AppCode))
	if err != nil {
		return CallbackPreview{}, errors.New(pbx.Describe(err))
	}
	plan, err := pbx.PlanInstall(st, c, queues)
	if err != nil {
		return CallbackPreview{}, err
	}
	return s.keep(p.ID, plan, route), nil
}

func (s *CallbackService) PreviewRemove(queues []string, dropWaiting bool) (CallbackPreview, error) {
	p, sec, err := s.profiles.active()
	if err != nil {
		return CallbackPreview{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), callbackReadTimeout)
	defer cancel()
	st, err := pbx.ReadCallbackState(ctx, p, sec, nil)
	if err != nil {
		return CallbackPreview{}, errors.New(pbx.Describe(err))
	}
	plan, err := pbx.PlanRemove(st, queues, dropWaiting)
	if err != nil {
		return CallbackPreview{}, err
	}
	return s.keep(p.ID, plan, ""), nil
}

// keep stores the preview Apply will run and counts its lines.
func (s *CallbackService) keep(profileID string, plan pbx.CallbackPlan, route string) CallbackPreview {
	s.mu.Lock()
	s.last = &callbackRun{profileID: profileID, plan: plan}
	s.mu.Unlock()
	pv := CallbackPreview{Plan: plan, Route: route}
	for _, st := range plan.Steps {
		switch st.Status {
		case pbx.CbNew:
			pv.New++
		case pbx.CbSkip:
			pv.Skip++
		case pbx.CbBlocked:
			pv.Blocked++
		}
	}
	return pv
}

func (s *CallbackService) Apply() (CallbackResult, error) {
	p, sec, err := s.profiles.active()
	if err != nil {
		return CallbackResult{}, err
	}
	run := s.take() // claimed once: any refusal from here on needs a new preview
	if err := run.check(p.ID); err != nil {
		return CallbackResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), callbackApplyTimeout)
	defer cancel()
	// Re-plan against a fresh read: write exactly what was previewed, or nothing.
	codes := pbx.CandidateCodes(run.plan.Config.AppCode)
	if run.plan.Remove {
		codes = nil
	}
	st, err := pbx.ReadCallbackState(ctx, p, sec, codes)
	if err != nil {
		return CallbackResult{}, errors.New(pbx.Describe(err))
	}
	fresh, err := replan(st, run.plan)
	if err != nil || !sameSteps(run.plan, fresh) {
		return CallbackResult{}, errChanged
	}
	cp, err := pbx.OpenCallbackPortal(ctx, p, sec)
	if err != nil {
		return CallbackResult{}, errors.New(pbx.Describe(err))
	}
	defer cp.Close()
	folio, err := s.profiles.NextFolio()
	if err != nil {
		return CallbackResult{}, err
	}
	progress := func(done int) { application.Get().Event.Emit("callback:avance", done) }
	steps := pbx.RunCallback(ctx, run.plan, pbx.CallbackStepRunner(p, sec, cp, run.plan), progress)
	vctx, vcancel := context.WithTimeout(context.Background(), callbackReadTimeout) // a long run must not starve the read-back
	defer vcancel()
	if after, err := pbx.ReadCallbackState(vctx, p, sec, nil); err == nil {
		steps = pbx.VerifyCallback(run.plan, steps, after)
	} else {
		for i := range steps {
			if steps[i].Status == pbx.CbApplied {
				steps[i].Status, steps[i].Error = pbx.CbFailed, "no se pudo verificar: "+pbx.Describe(err)
			}
		}
	}
	res := CallbackResult{Folio: folio, Steps: steps}
	for _, st := range steps {
		switch st.Status {
		case pbx.CbApplied:
			res.Applied++
		case pbx.CbSkipped:
			res.Skipped++
		case pbx.CbFailed:
			res.Failed++
		case pbx.CbNotRun:
			res.NotRun++
		}
	}
	return res, nil
}

// take returns the stored preview and clears it, so two Apply calls cannot both run it.
func (s *CallbackService) take() *callbackRun {
	s.mu.Lock()
	defer s.mu.Unlock()
	run := s.last
	s.last = nil
	return run
}

func replan(st pbx.CallbackState, prev pbx.CallbackPlan) (pbx.CallbackPlan, error) {
	if prev.Remove {
		return pbx.PlanRemove(st, prev.Queues, prev.DropWaiting)
	}
	return pbx.PlanInstall(st, prev.Config, prev.Queues)
}

// sameSteps: the same steps in the same order with the same codes and statuses (texts may differ).
func sameSteps(a, b pbx.CallbackPlan) bool {
	if len(a.Steps) != len(b.Steps) || a.Dialplan != b.Dialplan {
		return false
	}
	for i := range a.Steps {
		x, y := a.Steps[i], b.Steps[i]
		if x.Kind != y.Kind || x.Queue != y.Queue || x.Code != y.Code || x.Status != y.Status {
			return false
		}
	}
	return true
}

// check refuses an Apply that would not run exactly what the user previewed, or that has nothing to do.
func (r *callbackRun) check(profileID string) error {
	if r == nil {
		return errors.New("no hay vista previa: genérela primero")
	}
	if r.profileID != profileID {
		return errors.New("la vista previa no corresponde a la PBX actual: genérela de nuevo")
	}
	n := 0
	for _, st := range r.plan.Steps {
		switch st.Status {
		case pbx.CbBlocked:
			return errors.New("hay pasos bloqueados: quite esas colas de la selección")
		case pbx.CbNew:
			n++
		}
	}
	if n == 0 {
		return errors.New("no hay cambios para aplicar: todo ya está así")
	}
	return nil
}
