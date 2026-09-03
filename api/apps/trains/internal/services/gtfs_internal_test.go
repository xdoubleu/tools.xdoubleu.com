package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/trains/internal/mocks"
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
