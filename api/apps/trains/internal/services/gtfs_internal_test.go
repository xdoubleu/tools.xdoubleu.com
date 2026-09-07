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
		// publisher bug — 87:39:00 is 3.6 days, rejected.
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
	// the 87:39:00 row is dropped; the three valid rows survive.
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

	// matched by its full record_id: translations.txt supplies nl/en, and fr
	// comes from stop_name, the feed's primary language (feed_lang=fr).
	assert.Equal(t, "Brussel-Zuid", brusselsSouth.NameNL)
	assert.Equal(t, "Bruxelles-Midi", brusselsSouth.NameFR)
	assert.Equal(t, "Brussels-South", brusselsSouth.NameEN)

	// matched by a record_id the gateway's "gs:nmbssncb:" prefix was never
	// added to. The fr translation wins over stop_name even though fr is the
	// primary language — an explicit translation always beats the fallback.
	assert.Equal(t, "Anvers-Central", antwerp.NameFR)
	assert.Equal(t, "Antwerpen-Centraal", antwerp.NameNL)
	assert.Equal(t, "Antwerpen-Centraal", antwerp.NameEN)

	// matched by field_value rather than by any record id (issue #1459).
	assert.Equal(t, "Gand-Saint-Pierre", ghent.NameFR)
	assert.Equal(t, "Ghent-Sint-Pieters", ghent.NameEN)
	// nl is not translated for this stop, so it falls back to stop_name.
	assert.Equal(t, "Gent-Sint-Pieters", ghent.NameNL)

	// a field_value match is on the name, so the station's platform — which
	// shares that stop_name — is translated by the very same rows.
	assert.Equal(t, "Gand-Saint-Pierre", ghentPlatform.NameFR)
	assert.Equal(t, "Ghent-Sint-Pieters", ghentPlatform.NameEN)

	// a record_id match, by contrast, reaches only the record it names: the
	// station's platform keeps the untranslated stop_name in all three.
	assert.Equal(t, "Bruxelles-Midi", brusselsPlatform.NameNL)
	assert.Equal(t, "Bruxelles-Midi", brusselsPlatform.NameEN)
}

func TestParseFeed_TranslationCoverageIsReported(t *testing.T) {
	raw := mocks.BuildFeedZip(mocks.SampleFeedFiles())

	feed, err := parseFeed(logging.NewNopLogger(), raw)
	require.NoError(t, err)

	// The six usable stop_name rows in the fixture; every other row is
	// skipped before it is counted.
	assert.Equal(t, 6, feed.Info.Translations.Rows)
	// One of those names a stop this feed does not contain.
	assert.Equal(t, 1, feed.Info.Translations.RowsUnmatched)

	// Brussels-South (by record_id).
	assert.Equal(t, 1, feed.Info.Translations.StopsNL)
	// Antwerp (by bare record_id) plus Ghent's station and its platform
	// (both by field_value).
	assert.Equal(t, 3, feed.Info.Translations.StopsFR)
	// Brussels-South plus Ghent's station and its platform.
	assert.Equal(t, 3, feed.Info.Translations.StopsEN)
}

// A feed whose translations.txt keys rows by an id this feed's stops.txt
// never uses is exactly the production failure of issue #1459: the import
// succeeds, every name silently stays monolingual, and only the coverage
// counters say so.
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

// record_id and field_value are mutually exclusive in GTFS; a publisher that
// sets both gets the more specific of the two.
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
	// a bare quote mid-field is a real encoding/csv parse error, distinct
	// from the io.EOF that ends a well-formed file.
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

// A stops.txt row with no stop_id is skipped rather than stored under an
// empty key, where it would collide with every other such row.
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

// A stop with a blank stop_name has nothing for a field_value row to match
// on, so that candidate is skipped rather than looked up under the empty
// key — which would otherwise collide with every other unnamed stop.
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
	// The row identified neither a record nor a value, so it was never
	// indexed and cannot be reported unmatched either.
	assert.Equal(t, 0, feed.Info.Translations.Rows)
}

// A malformed stops.txt is a real parse error, not a skipped row: failing
// the whole import beats silently storing a truncated timetable.
func TestParseFeed_MalformedStopsRowPropagatesError(t *testing.T) {
	files := mocks.SampleFeedFiles()
	files["stops.txt"] = "location_type,parent_station,platform_code,stop_id," +
		"stop_lat,stop_lon,stop_name\n" +
		"1,,,gs:nmbssncb:S8814001,50.83,4.33,broken\"value\n"

	_, err := parseFeed(logging.NewNopLogger(), mocks.BuildFeedZip(files))
	require.Error(t, err)
}
