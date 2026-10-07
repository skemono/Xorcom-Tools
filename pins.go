package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/skemono/Xorcom-Tools/pbx"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// PinService is F-02: add PINs from a CSV to an existing PIN list of the active PBX.
type PinService struct {
	profiles *ProfileService
	mu       sync.Mutex
	last     *pinPlan // exactly what Apply writes: the last preview
}

type pinPlan struct {
	profileID string
	listID    int
	rows      []pbx.PinRow
}

type PinPreview struct {
	ListID   int          `json:"listID"`
	Info     pbx.CSVInfo  `json:"info"`
	Rows     []pbx.PinRow `json:"rows"`
	New      int          `json:"new"`
	Existing int          `json:"existing"`
	NoDesc   int          `json:"noDesc"`
	Errors   int          `json:"errors"`
}

type PinApplyResult struct {
	Folio   int          `json:"folio"`
	Rows    []pbx.PinRow `json:"rows"`
	Applied int          `json:"applied"`
	Skipped int          `json:"skipped"`
	Failed  int          `json:"failed"`
}

const (
	pinTimeout   = 60 * time.Second
	applyTimeout = 5 * time.Minute // one NOT EXISTS lookup per row; the real list is 5 772 rows
	maxCSV       = 1 << 20
)

func (s *PinService) Lists() ([]pbx.PinList, error) {
	p, sec, err := s.profiles.active()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), pinTimeout)
	defer cancel()
	lists, err := pbx.PinLists(ctx, p, sec)
	if err != nil {
		return nil, errors.New(pbx.Describe(err))
	}
	return lists, nil
}

// PickCSV opens the native file dialog; "" means the user cancelled.
func (s *PinService) PickCSV() (string, error) {
	return application.Get().Dialog.OpenFile().
		SetTitle("Archivo CSV de PINes").
		AddFilter("CSV (*.csv)", "*.csv").
		PromptForSingleSelection()
}

func (s *PinService) Preview(listID int, path string, includeEmpty bool) (PinPreview, error) {
	p, sec, err := s.profiles.active()
	if err != nil {
		return PinPreview{}, err
	}
	st, err := os.Stat(path)
	if err != nil {
		return PinPreview{}, fmt.Errorf("no se pudo abrir el archivo: %v", err)
	}
	if st.Size() > maxCSV {
		return PinPreview{}, errors.New("el archivo pasa de 1 MB: no parece una lista de PINes")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return PinPreview{}, fmt.Errorf("no se pudo leer el archivo: %v", err)
	}
	info, rows, err := pbx.ParsePinCSV(raw, listID)
	if err != nil {
		return PinPreview{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), pinTimeout)
	defer cancel()
	existing, err := pbx.PinEntries(ctx, p, sec, listID)
	if err != nil {
		return PinPreview{}, errors.New(pbx.Describe(err))
	}
	rows = pbx.PlanPins(rows, existing, includeEmpty)
	s.mu.Lock()
	s.last = &pinPlan{profileID: p.ID, listID: listID, rows: rows}
	s.mu.Unlock()
	pv := PinPreview{ListID: listID, Info: info, Rows: rows}
	for _, r := range rows {
		switch r.Status {
		case pbx.PinNew:
			pv.New++
		case pbx.PinExists:
			pv.Existing++
		case pbx.PinNoDesc:
			pv.NoDesc++
		case pbx.PinError:
			pv.Errors++
		}
	}
	return pv, nil
}

func (s *PinService) Apply(listID int) (PinApplyResult, error) {
	p, sec, err := s.profiles.active()
	if err != nil {
		return PinApplyResult{}, err
	}
	s.mu.Lock()
	plan := s.last
	s.mu.Unlock()
	if err := plan.check(p.ID, listID); err != nil {
		return PinApplyResult{}, err
	}
	folio, err := s.profiles.NextFolio()
	if err != nil {
		return PinApplyResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), applyTimeout)
	defer cancel()
	res := PinApplyResult{Folio: folio, Rows: pbx.ApplyPins(ctx, p, sec, listID, plan.rows)}
	for _, r := range res.Rows {
		switch r.Status {
		case pbx.PinApplied:
			res.Applied++
		case pbx.PinSkipped:
			res.Skipped++
		case pbx.PinFailed:
			res.Failed++
		}
	}
	if res.Failed == 0 {
		s.mu.Lock()
		s.last = nil // done; a second Aplicar needs a fresh preview
		s.mu.Unlock()
	}
	return res, nil
}

// ApplyPortal runs the portal's own Apply with the profile's API credentials (F-01). It reloads
// the PBX with ALL outstanding portal changes; the UI only calls it after a two-step confirmation.
func (s *PinService) ApplyPortal() (string, error) {
	p, sec, err := s.profiles.active()
	if err != nil {
		return "", err
	}
	if !p.API.Enabled || sec.API == "" {
		return "", errors.New("falta la contraseña del portal: agréguela en F-01 (canal API)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), pinTimeout)
	defer cancel()
	msg, err := pbx.PortalApplyChanges(ctx, p, sec.API)
	if err != nil {
		return "", errors.New(pbx.Describe(err))
	}
	return msg, nil
}

// check refuses an Apply that would not write exactly what the user previewed.
func (pl *pinPlan) check(profileID string, listID int) error {
	if pl == nil {
		return errors.New("no hay vista previa: cargue el archivo primero")
	}
	if pl.profileID != profileID || pl.listID != listID {
		return errors.New("la vista previa no corresponde a la PBX y lista actuales: cárguela de nuevo")
	}
	n, bad := 0, 0
	for _, r := range pl.rows {
		switch r.Status {
		case pbx.PinNew:
			n++
		case pbx.PinError:
			bad++
		}
	}
	if bad > 0 {
		return fmt.Errorf("hay %d fila(s) con error: corrija el archivo y vuelva a cargarlo", bad)
	}
	if n == 0 {
		return errors.New("no hay PINes nuevos para aplicar")
	}
	return nil
}
