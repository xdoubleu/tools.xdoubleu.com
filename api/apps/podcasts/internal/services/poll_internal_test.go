package services

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/podcasts/internal/models"
)

// item builds a feed item with only the fields a test sets.
func item(set func(*gofeed.Item)) *gofeed.Item {
	i := &gofeed.Item{} //nolint:exhaustruct // tests set what they read
	if set != nil {
		set(i)
	}
	return i
}

func itemWithDuration(d string) *gofeed.Item {
	return item(func(i *gofeed.Item) {
		//nolint:exhaustruct // only Duration is read
		i.ITunesExt = &ext.ITunesItemExtension{Duration: d}
	})
}

func TestDuration(t *testing.T) {
	tests := map[string]*int{
		"3725":        ptr(3725),
		"62:15":       ptr(3735),
		"01:02:03":    ptr(3723),
		"\t90":        ptr(90),
		"":            nil,
		"soon":        nil,
		"1:xx":        nil,
		"-5":          nil,
		"0:30":        ptr(30),
		"0":           ptr(0),
		"2147483647":  ptr(2147483647),
		"2147483648":  nil,
		"99999999:00": nil,
	}
	for raw, want := range tests {
		t.Run(raw, func(t *testing.T) {
			assert.Equal(t, want, duration(itemWithDuration(raw)))
		})
	}
	assert.Nil(t, duration(item(nil)), "no iTunes extension")
}

func TestSummarize(t *testing.T) {
	assert.Equal(t, "A & B C", summarize("<p>A &amp; <b>B</b></p>\n\n  C "))
	assert.Empty(t, summarize("   "))

	long := summarize(strings.Repeat("é", maxSummaryRunes+50))
	assert.Equal(t, maxSummaryRunes+1, len([]rune(long)))
	assert.True(t, strings.HasSuffix(long, "…"))

	exact := strings.Repeat("a", maxSummaryRunes)
	assert.Equal(t, exact, summarize(exact))
}

func TestItemGUID_FallsBackStepByStep(t *testing.T) {
	enc := []*gofeed.Enclosure{{URL: "https://cdn/a.mp3", Length: "", Type: ""}}
	assert.Equal(t, "g", itemGUID(item(func(i *gofeed.Item) {
		i.GUID, i.Enclosures, i.Link, i.Title = " g ", enc, "l", "t"
	})))
	assert.Equal(t, "https://cdn/a.mp3", itemGUID(item(func(i *gofeed.Item) {
		i.Enclosures, i.Link = enc, "l"
	})))
	assert.Equal(
		t,
		"l",
		itemGUID(item(func(i *gofeed.Item) { i.Link, i.Title = "l", "t" })),
	)
	assert.Equal(t, "t", itemGUID(item(func(i *gofeed.Item) { i.Title = "t" })))
	assert.Empty(t, itemGUID(item(nil)))
}

func TestAudioURL_SkipsEmptyEnclosures(t *testing.T) {
	withEnclosures := item(func(i *gofeed.Item) {
		i.Enclosures = []*gofeed.Enclosure{
			nil,
			{URL: "", Length: "", Type: ""},
			{URL: "u", Length: "", Type: ""},
		}
	})
	assert.Equal(t, "u", audioURL(withEnclosures))
}

func TestToEpisodes_SkipsItemsWithoutIdentityAndCapsTheCount(t *testing.T) {
	show := models.Show{} //nolint:exhaustruct // only the ID is read
	show.ID = uuid.New()
	items := []*gofeed.Item{item(nil)}
	for i := range maxEpisodesPerPoll + 20 {
		items = append(
			items,
			item(func(it *gofeed.Item) { it.GUID = strings.Repeat("g", i+1) }),
		)
	}

	got := toEpisodes(show, items)
	require.Len(t, got, maxEpisodesPerPoll)
	assert.Equal(t, show.ID, got[0].ShowID)
	assert.Equal(t, uuid.Nil, got[0].ID)
	assert.Equal(t, "g", got[0].GUID, "the empty item was skipped")
}

func ptr(n int) *int { return &n }

func dated(title string, day int) *gofeed.Item {
	return item(func(i *gofeed.Item) {
		i.GUID, i.Title = title, title
		if day > 0 {
			d := time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC)
			i.PublishedParsed = &d
		}
	})
}

func TestToEpisodes_KeepsTheNewestWhicheverEndTheFeedListsFirst(t *testing.T) {
	show := models.Show{} //nolint:exhaustruct // only the ID is read
	var oldestFirst []*gofeed.Item
	for day := 1; day <= 28; day++ {
		oldestFirst = append(oldestFirst, dated(fmt.Sprintf("d%02d", day), day))
	}
	// Repeat across months until the cap bites.
	for i := range maxEpisodesPerPoll {
		d := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i)
		oldestFirst = append([]*gofeed.Item{item(func(it *gofeed.Item) {
			it.GUID, it.PublishedParsed = fmt.Sprintf("old%03d", i), &d
		})}, oldestFirst...)
	}

	got := toEpisodes(show, oldestFirst)
	require.Len(t, got, maxEpisodesPerPoll)
	assert.Equal(t, "d28", got[0].GUID, "newest first")
	assert.Equal(t, "d01", got[27].GUID)
	for _, e := range got {
		assert.NotEqual(t, "old000", e.GUID, "the oldest was dropped by the cap")
	}
}

func TestToEpisodes_UndatedItemsSortLastAndKeepTheirOrder(t *testing.T) {
	show := models.Show{} //nolint:exhaustruct // only the ID is read
	got := toEpisodes(show, []*gofeed.Item{
		dated("undated-a", 0), dated("old", 1), dated("undated-b", 0), dated("new", 2),
	})
	guids := make([]string, len(got))
	for i, e := range got {
		guids[i] = e.GUID
	}
	assert.Equal(t, []string{"new", "old", "undated-a", "undated-b"}, guids)
}

func TestWebURL(t *testing.T) {
	tests := map[string]string{
		"https://shows.example/a?b=c": "https://shows.example/a?b=c",
		"\thttp://shows.example/a":    "http://shows.example/a",
		"javascript:alert(1)":         "",
		"data:text/html,hi":           "",
		"ftp://shows.example/a":       "",
		"//shows.example/a":           "",
		"/relative":                   "",
		"":                            "",
		"http://[::1":                 "",
	}
	for raw, want := range tests {
		t.Run(raw, func(t *testing.T) {
			assert.Equal(t, want, webURL(raw))
		})
	}
}

func TestToEpisodes_DropsNonWebLinks(t *testing.T) {
	show := models.Show{} //nolint:exhaustruct // only the ID is read
	got := toEpisodes(show, []*gofeed.Item{item(func(i *gofeed.Item) {
		i.GUID, i.Link = "g", "javascript:alert(1)"
		i.Enclosures = []*gofeed.Enclosure{{URL: "data:audio/mp3,x", Length: "", Type: ""}}
	})})
	require.Len(t, got, 1)
	assert.Empty(t, got[0].Link)
	assert.Empty(t, got[0].AudioURL)
}
