package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"tools.xdoubleu.com/apps/trains/pkg/irail"
)

// ErrInvalidDate is a caller-fixable date that isn't YYYY-MM-DD.
var ErrInvalidDate = errors.New("invalid date")

// IrailDiagnostic is iRail's request health, plus one train's raw response
// when a vehicle was asked for.
type IrailDiagnostic struct {
	Diagnostics irail.Diagnostics `json:"diagnostics"`
	Vehicle     map[string]any    `json:"vehicle,omitempty"`
}

// IrailService traces a wrong delay to iRail versus our own mapping.
type IrailService struct {
	client irail.Client
	now    func() time.Time
}

func NewIrailService(client irail.Client) *IrailService {
	return &IrailService{client: client, now: time.Now}
}

// Diagnostic reports request health; with a vehicleID (date YYYY-MM-DD, empty
// for today) it adds iRail's raw response for that train. An iRail failure is
// reported in the result rather than failing the whole diagnostic.
func (s *IrailService) Diagnostic(
	ctx context.Context, vehicleID, date string,
) (IrailDiagnostic, error) {
	out := IrailDiagnostic{Diagnostics: s.client.Diagnostics(), Vehicle: nil}
	if vehicleID == "" {
		return out, nil
	}

	day := s.now()
	if date != "" {
		parsed, err := time.Parse(time.DateOnly, date)
		if err != nil {
			return IrailDiagnostic{}, fmt.Errorf(
				"%w: date must be YYYY-MM-DD: %w", ErrInvalidDate, err,
			)
		}
		day = parsed
	}

	id := irail.VehicleID(vehicleID)
	out.Vehicle = map[string]any{"id": id}
	raw, err := s.client.RawVehicle(ctx, id, day)
	if err != nil {
		out.Vehicle["error"] = err.Error()
		return out, nil //nolint:nilerr //the failure is the diagnostic
	}
	var decoded any
	if err = json.Unmarshal(raw, &decoded); err != nil {
		out.Vehicle["error"] = err.Error()
		return out, nil //nolint:nilerr //the failure is the diagnostic
	}
	out.Vehicle["raw"] = decoded
	return out, nil
}
