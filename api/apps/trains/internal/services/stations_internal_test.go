package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

// mkStop builds a station/platform fixture.
//
//nolint:exhaustruct //test fixture: unset fields are deliberately zero
func mkStop(
	stopID, nameNL, nameFR, nameEN string,
	locationType int,
	uic string,
) models.Stop {
	return models.Stop{
		StopID:       stopID,
		NameNL:       nameNL,
		NameFR:       nameFR,
		NameEN:       nameEN,
		LocationType: locationType,
		UIC:          uic,
	}
}

func TestMatchStations_DedupesByUIC(t *testing.T) {
	stops := []models.Stop{
		mkStop(
			"gs:nmbssncb:S8896800", "Roeselare", "Roulers", "Roeselare / Roulers",
			stationLocationType, "8896800",
		),
		mkStop(
			"gs:nmbssncb:S8896800B", "Roeselare", "Roulers", "Roeselare / Roulers",
			stationLocationType, "8896800",
		),
	}

	got := matchStations(stops, "Roeselare")

	assert.Len(t, got, 1)
	assert.Equal(t, "gs:nmbssncb:S8896800", got[0].StopID)
}

func TestMatchStations_EmptyUICNeverMerged(t *testing.T) {
	stops := []models.Stop{
		mkStop("weird-id-1", "Foo", "Foo", "Foo", stationLocationType, ""),
		mkStop("weird-id-2", "Bar", "Bar", "Bar", stationLocationType, ""),
	}

	got := matchStations(stops, "")

	assert.Len(t, got, 2)
}

func TestMatchStations_ExcludesPlatformsAndFiltersByQuery(t *testing.T) {
	stops := []models.Stop{
		mkStop(
			"gs:nmbssncb:S1", "Gent-Sint-Pieters", "Gand-Saint-Pierre",
			"Gand-Saint-Pierre / Gent-Sint-Pieters", stationLocationType, "1",
		),
		// platform, not a station.
		mkStop(
			"gs:nmbssncb:1_1", "Gent-Sint-Pieters", "Gand-Saint-Pierre",
			"Gand-Saint-Pierre / Gent-Sint-Pieters", 0, "1",
		),
		mkStop(
			"gs:nmbssncb:S2", "Brussel-Zuid", "Bruxelles-Midi",
			"Brux.-Midi/Brus.-Zuid", stationLocationType, "2",
		),
	}

	got := matchStations(stops, "gent")

	assert.Len(t, got, 1)
	assert.Equal(t, "gs:nmbssncb:S1", got[0].StopID)
}
