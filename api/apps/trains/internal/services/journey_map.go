package services

import (
	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/internal/repositories"
	"tools.xdoubleu.com/apps/trains/pkg/csa"
)

func toCSAStops(stops []models.Stop) []csa.StopInput {
	out := make([]csa.StopInput, len(stops))
	for i, s := range stops {
		out[i] = csa.StopInput{
			ID:            s.StopID,
			Name:          s.Name,
			ParentStation: s.ParentStation,
			PlatformCode:  s.PlatformCode,
			LocationType:  s.LocationType,
		}
	}
	return out
}

func toCSATransfers(transfers []models.Transfer) []csa.TransferInput {
	out := make([]csa.TransferInput, len(transfers))
	for i, t := range transfers {
		out[i] = csa.TransferInput{
			FromStopID:      t.FromStopID,
			ToStopID:        t.ToStopID,
			TransferType:    t.TransferType,
			MinTransferTime: t.MinTransferTime,
		}
	}
	return out
}

func toCSAInstances(active []repositories.ActiveTrip) []csa.TripInstanceInput {
	out := make([]csa.TripInstanceInput, len(active))
	for i, a := range active {
		out[i] = csa.TripInstanceInput{
			TripID:         a.TripID,
			ShortName:      a.TripShortName,
			RouteShortName: a.RouteShortName,
			Headsign:       a.TripHeadsign,
			Date:           a.Date,
		}
	}
	return out
}

func toCSAPatterns(stopTimes []models.StopTime) map[string][]csa.StopTimeInput {
	out := make(map[string][]csa.StopTimeInput)
	for _, st := range stopTimes {
		out[st.TripID] = append(out[st.TripID], csa.StopTimeInput{
			StopSequence:     st.StopSequence,
			StopID:           st.StopID,
			ArrivalSeconds:   st.ArrivalSeconds,
			DepartureSeconds: st.DepartureSeconds,
			PickupType:       st.PickupType,
			DropOffType:      st.DropOffType,
		})
	}
	return out
}
