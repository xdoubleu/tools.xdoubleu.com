package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/mocks"
	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/internal/logging"
)

func TestParseGTFSTime(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"08:30:00", 8*3600 + 30*60, true},
		{"00:00:00", 0, true},
		// > 24h is legal GTFS for after-midnight service.
		{"25:15:00", 25*3600 + 15*60, true},
		{"36:00:00", 36 * 3600, true},
		// Publisher bug: 3.6 days, rejected.
		{"87:39:00", 0, false},
		{"", 0, false},
		{"8:30", 0, false},
		{"aa:bb:cc", 0, false},
		{"10:75:00", 0, false},
	}
	for _, c := range cases {
		got, ok := parseGTFSTime(c.in)
		assert.Equal(t, c.ok, ok, c.in)
		assert.Equal(t, c.want, got, c.in)
	}
}

func TestUICFromStopID(t *testing.T) {
	assert.Equal(t, "8814001", uicFromStopID("gs:nmbssncb:S8814001"))
	assert.Equal(t, "8814001", uicFromStopID("gs:nmbssncb:8814001"))
	assert.Equal(t, "8814001", uicFromStopID("gs:nmbssncb:8814001_10"))
	// an unassigned-platform suffix still resolves to its station UIC
	assert.Equal(t, "8814001", uicFromStopID("gs:nmbssncb:8814001_TE BEPAL"))
	assert.Equal(t, "", uicFromStopID("weird-id"))
}

func TestParseFeed_RejectsNonZip(t *testing.T) {
	_, err := parseFeed(logging.NewNopLogger(), []byte("<html>not a zip</html>"))
	assert.ErrorIs(t, err, errZipMagic)
}

func TestParseFeed_MissingFileIsError(t *testing.T) {
	files := mocks.SampleFeedFiles()
	delete(files, "stop_times.txt")
	_, err := parseFeed(logging.NewNopLogger(), mocks.BuildFeedZip(files))
	require.ErrorContains(t, err, "stop_times.txt missing")
}

func TestParseFeed_MissingStopsFileIsError(t *testing.T) {
	files := mocks.SampleFeedFiles()
	delete(files, "stops.txt")
	_, err := parseFeed(logging.NewNopLogger(), mocks.BuildFeedZip(files))
	require.ErrorContains(t, err, "stops.txt missing")
}

func TestParseFeed_NoFeedVersionIsError(t *testing.T) {
	files := mocks.SampleFeedFiles()
	files["feed_info.txt"] = "feed_lang,feed_version\nfr,\n"
	_, err := parseFeed(logging.NewNopLogger(), mocks.BuildFeedZip(files))
	require.ErrorContains(t, err, "feed_version")
}

func TestParseGTFSDate(t *testing.T) {
	d, ok := parseGTFSDate("20260831")
	require.True(t, ok)
	assert.Equal(t, 2026, d.Year())
	_, ok = parseGTFSDate("not-a-date")
	assert.False(t, ok)
}

func TestParseFeed_TrapsAndBounds(t *testing.T) {
	raw := mocks.BuildFeedZip(mocks.SampleFeedFiles())

	feed, err := parseFeed(logging.NewNopLogger(), raw)
	require.NoError(t, err)

	assert.Equal(t, "2026-08-31", feed.Info.FeedVersion)
	assert.NotEmpty(t, feed.CalendarDates)
	for _, cd := range feed.CalendarDates {
		assert.Equal(t, 1, cd.ExceptionType)
	}
	assert.Len(t, feed.StopTimes, 3)

	var nonBoarding int
	for _, st := range feed.StopTimes {
		if st.PickupType == 1 && st.DropOffType == 1 {
			nonBoarding++
		}
	}
	assert.Equal(t, 1, nonBoarding)

	assert.Equal(t, "8814001", feed.Stops[0].UIC)
}

func TestParseFeed_StopNamesMultilingual(t *testing.T) {
	raw := mocks.BuildFeedZip(mocks.SampleFeedFiles())

	feed, err := parseFeed(logging.NewNopLogger(), raw)
	require.NoError(t, err)

	var brusselsSouth, ghent, ghentPlatform, antwerp, brusselsPlatform models.Stop
	for _, s := range feed.Stops {
		switch s.StopID {
		case "gs:nmbssncb:S8814001":
			brusselsSouth = s
		case "gs:nmbssncb:8814001_3":
			brusselsPlatform = s
		case "gs:nmbssncb:S8892007":
			ghent = s
		case "gs:nmbssncb:8892007_1":
			ghentPlatform = s
		case "gs:nmbssncb:8821006":
			antwerp = s
		}
	}

	// Full record_id match; fr comes from the primary stop_name.
	assert.Equal(t, "Brussel-Zuid", brusselsSouth.NameNL)
	assert.Equal(t, "Bruxelles-Midi", brusselsSouth.NameFR)
	assert.Equal(t, "Brussels-South", brusselsSouth.NameEN)

	// Bare record_id match. An explicit fr translation beats the primary
	// stop_name.
	assert.Equal(t, "Anvers-Central", antwerp.NameFR)
	assert.Equal(t, "Antwerpen-Centraal", antwerp.NameNL)
	assert.Equal(t, "Antwerpen-Centraal", antwerp.NameEN)

	// field_value match.
	assert.Equal(t, "Gand-Saint-Pierre", ghent.NameFR)
	assert.Equal(t, "Ghent-Sint-Pieters", ghent.NameEN)
	assert.Equal(t, "Gent-Sint-Pieters", ghent.NameNL)

	// field_value matches the name, so the platform sharing it is translated too.
	assert.Equal(t, "Gand-Saint-Pierre", ghentPlatform.NameFR)
	assert.Equal(t, "Ghent-Sint-Pieters", ghentPlatform.NameEN)

	// A record_id match reaches only the record it names.
	assert.Equal(t, "Bruxelles-Midi", brusselsPlatform.NameNL)
	assert.Equal(t, "Bruxelles-Midi", brusselsPlatform.NameEN)

	// DisplayName only shows genuinely-known names: Ghent has no nl translation;
	// Antwerp's fr translation replaces its raw stop_name.
	assert.Equal(t, "Gand-Saint-Pierre / Ghent-Sint-Pieters", ghent.DisplayName)
	assert.Equal(t, "Anvers-Central", antwerp.DisplayName)
}

// TestParseFeed_DisplayNameDedupesAbbreviatedFallback: an abbreviated
// bilingual raw stop_name must not leak into DisplayName.
func TestParseFeed_DisplayNameDedupesAbbreviatedFallback(t *testing.T) {
	files := mocks.SampleFeedFiles()
	files["stops.txt"] += "1,,,gs:nmbssncb:S8811305,50.85,4.35,Brsls Centr / Bxl Centr\n"
	files["translations.txt"] += "stop_name,,nl,gs:nmbssncb:S8811305,stops," +
		"Brussel-Centraal\n" +
		"stop_name,,fr,gs:nmbssncb:S8811305,stops,Bruxelles-Central\n"

	raw := mocks.BuildFeedZip(files)
	feed, err := parseFeed(logging.NewNopLogger(), raw)
	require.NoError(t, err)

	var brusselsCentral models.Stop
	for _, s := range feed.Stops {
		if s.StopID == "gs:nmbssncb:S8811305" {
			brusselsCentral = s
		}
	}

	assert.Equal(t, "Bruxelles-Central / Brussel-Centraal", brusselsCentral.DisplayName)
	assert.NotContains(t, brusselsCentral.DisplayName, "Brsls Centr")
	assert.NotContains(t, brusselsCentral.DisplayName, "Bxl Centr")
}

// TestParseFeed_DisplayNameKeepsDistinctFullNames: distinct full names in two
// languages both show.
func TestParseFeed_DisplayNameKeepsDistinctFullNames(t *testing.T) {
	files := mocks.SampleFeedFiles()
	files["stops.txt"] += "1,,,gs:nmbssncb:S8896008,50.95,3.13,Roulers\n"
	files["translations.txt"] += "stop_name,,nl,gs:nmbssncb:S8896008,stops,Roeselare\n"

	raw := mocks.BuildFeedZip(files)
	feed, err := parseFeed(logging.NewNopLogger(), raw)
	require.NoError(t, err)

	var roeselare models.Stop
	for _, s := range feed.Stops {
		if s.StopID == "gs:nmbssncb:S8896008" {
			roeselare = s
		}
	}
	assert.Equal(t, "Roulers / Roeselare", roeselare.DisplayName)
}

// TestParseFeed_DisplayNameRejectsSyntheticCombinedTranslation: a genuine
// translations.txt row holding "{French} / {Dutch}" is not a DisplayName part.
func TestParseFeed_DisplayNameRejectsSyntheticCombinedTranslation(t *testing.T) {
	files := mocks.SampleFeedFiles()
	files["stops.txt"] += "1,,,gs:nmbssncb:S8813003,50.85,4.36,Bruxelles-Central\n"
	files["translations.txt"] += "stop_name,,nl,gs:nmbssncb:S8813003,stops," +
		"Brussel-Centraal\n" +
		"stop_name,,en,gs:nmbssncb:S8813003,stops,Brux.-/ Brus-Centr.\n"
	files["stops.txt"] += "1,,,gs:nmbssncb:S8896800,50.95,3.13,Roulers\n"
	files["translations.txt"] += "stop_name,,nl,gs:nmbssncb:S8896800,stops," +
		"Roeselare\n" +
		"stop_name,,en,gs:nmbssncb:S8896800,stops,Roeselare / Roulers\n"

	raw := mocks.BuildFeedZip(files)
	feed, err := parseFeed(logging.NewNopLogger(), raw)
	require.NoError(t, err)

	var brusselsCentral, roeselare models.Stop
	for _, s := range feed.Stops {
		switch s.StopID {
		case "gs:nmbssncb:S8813003":
			brusselsCentral = s
		case "gs:nmbssncb:S8896800":
			roeselare = s
		}
	}

	assert.Equal(t, "Bruxelles-Central / Brussel-Centraal", brusselsCentral.DisplayName)
	assert.NotContains(t, brusselsCentral.DisplayName, "Brux.-/ Brus-Centr.")

	assert.Equal(t, "Roulers / Roeselare", roeselare.DisplayName)
	assert.NotContains(t, roeselare.DisplayName, "Roeselare / Roulers")
}

func TestParseFeed_TranslationCoverageIsReported(t *testing.T) {
	raw := mocks.BuildFeedZip(mocks.SampleFeedFiles())

	feed, err := parseFeed(logging.NewNopLogger(), raw)
	require.NoError(t, err)

	// The fixture's six usable stop_name rows.
	assert.Equal(t, 6, feed.Info.Translations.Rows)
	assert.Equal(t, 1, feed.Info.Translations.RowsUnmatched)

	assert.Equal(t, 1, feed.Info.Translations.StopsNL)
	// Antwerp by bare record_id, Ghent station and platform by field_value.
	assert.Equal(t, 3, feed.Info.Translations.StopsFR)
	assert.Equal(t, 3, feed.Info.Translations.StopsEN)
}

// TestParseFeed_UnmatchedTranslationsAreCountedNotSilent: rows keyed by ids
// the feed never uses must show in the coverage counters.
func TestParseFeed_UnmatchedTranslationsAreCountedNotSilent(t *testing.T) {
	files := mocks.SampleFeedFiles()
	files["translations.txt"] = "field_name,field_value,language,record_id," +
		"table_name,translation\n" +
		"stop_name,,nl,no-such-stop-1,stops,Brussel-Zuid\n" +
		"stop_name,,en,no-such-stop-2,stops,Brussels-South\n"

	feed, err := parseFeed(logging.NewNopLogger(), mocks.BuildFeedZip(files))
	require.NoError(t, err)

	for _, s := range feed.Stops {
		assert.Equal(t, s.NameFR, s.NameNL)
		assert.Equal(t, s.NameFR, s.NameEN)
	}
	assert.Equal(t, 2, feed.Info.Translations.Rows)
	assert.Equal(t, 2, feed.Info.Translations.RowsUnmatched)
	assert.Equal(t, 0, feed.Info.Translations.StopsNL)
	assert.Equal(t, 0, feed.Info.Translations.StopsEN)
}

// TestParseFeed_RecordIDWinsOverFieldValue: record_id wins when both are set.
func TestParseFeed_RecordIDWinsOverFieldValue(t *testing.T) {
	files := mocks.SampleFeedFiles()
	files["translations.txt"] = "field_name,field_value,language,record_id," +
		"table_name,translation\n" +
		"stop_name,Bruxelles-Midi,nl,gs:nmbssncb:S8814001,stops,By-Record-ID\n" +
		"stop_name,Bruxelles-Midi,en,,stops,By-Field-Value\n"

	feed, err := parseFeed(logging.NewNopLogger(), mocks.BuildFeedZip(files))
	require.NoError(t, err)

	for _, s := range feed.Stops {
		if s.StopID == "gs:nmbssncb:S8814001" {
			assert.Equal(t, "By-Record-ID", s.NameNL)
			assert.Equal(t, "By-Field-Value", s.NameEN)
		}
	}
}

func TestParseTranslations_MalformedRowPropagatesError(t *testing.T) {
	files := mocks.SampleFeedFiles()
	// A bare quote mid-field is a real csv error, not io.EOF.
	files["translations.txt"] = "field_name,field_value,language,record_id," +
		"table_name,translation\n" +
		"stop_name,,nl,gs:nmbssncb:S8814001,stops,broken\"value\n"

	_, err := parseFeed(logging.NewNopLogger(), mocks.BuildFeedZip(files))
	require.Error(t, err)
}

func TestParseFeed_MissingTranslationsIsNotAnError(t *testing.T) {
	files := mocks.SampleFeedFiles()
	delete(files, "translations.txt")

	feed, err := parseFeed(logging.NewNopLogger(), mocks.BuildFeedZip(files))
	require.NoError(t, err)

	for _, s := range feed.Stops {
		assert.Equal(t, s.NameFR, s.NameNL)
		assert.Equal(t, s.NameFR, s.NameEN)
	}
	assert.Equal(t, 0, feed.Info.Translations.Rows)
	assert.Equal(t, 0, feed.Info.Translations.RowsUnmatched)
}

// TestParseFeed_StopWithoutIDIsSkipped: no empty-key collisions.
func TestParseFeed_StopWithoutIDIsSkipped(t *testing.T) {
	files := mocks.SampleFeedFiles()
	files["stops.txt"] = "location_type,parent_station,platform_code,stop_id," +
		"stop_lat,stop_lon,stop_name\n" +
		"1,,,gs:nmbssncb:S8814001,50.83,4.33,Bruxelles-Midi\n" +
		"1,,,,50.00,4.00,No-Stop-ID\n"

	feed, err := parseFeed(logging.NewNopLogger(), mocks.BuildFeedZip(files))
	require.NoError(t, err)

	require.Len(t, feed.Stops, 1)
	assert.Equal(t, "gs:nmbssncb:S8814001", feed.Stops[0].StopID)
}

// TestParseFeed_BlankStopNameMatchesNoFieldValueRow: no empty-key lookup.
func TestParseFeed_BlankStopNameMatchesNoFieldValueRow(t *testing.T) {
	files := mocks.SampleFeedFiles()
	files["stops.txt"] = "location_type,parent_station,platform_code,stop_id," +
		"stop_lat,stop_lon,stop_name\n" +
		"1,,,gs:nmbssncb:S8814001,50.83,4.33,\n"
	files["translations.txt"] = "field_name,field_value,language,record_id," +
		"table_name,translation\n" +
		"stop_name,,nl,,stops,Should-Not-Be-Applied\n"

	feed, err := parseFeed(logging.NewNopLogger(), mocks.BuildFeedZip(files))
	require.NoError(t, err)

	require.Len(t, feed.Stops, 1)
	assert.Empty(t, feed.Stops[0].NameNL)
	// Neither a record nor a value, so never indexed.
	assert.Equal(t, 0, feed.Info.Translations.Rows)
}

// TestParseFeed_MalformedStopsRowPropagatesError: fail rather than store a
// truncated timetable.
func TestParseFeed_MalformedStopsRowPropagatesError(t *testing.T) {
	files := mocks.SampleFeedFiles()
	files["stops.txt"] = "location_type,parent_station,platform_code,stop_id," +
		"stop_lat,stop_lon,stop_name\n" +
		"1,,,gs:nmbssncb:S8814001,50.83,4.33,broken\"value\n"

	_, err := parseFeed(logging.NewNopLogger(), mocks.BuildFeedZip(files))
	require.Error(t, err)
}
