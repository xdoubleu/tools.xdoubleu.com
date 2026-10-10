//nolint:testpackage // testing unexported Kobo store parsing
package books

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKoboStoreItems(t *testing.T) {
	items := []json.RawMessage{
		json.RawMessage(`{"NewEntitlement":{
			"BookEntitlement":{"Id":"a","Accessibility":"Full","OriginCategory":"Purchased"},
			"BookMetadata":{"Title":"A","Isbn":"9780000000001","Contributors":["X"]},
			"ReadingState":{"CurrentBookmark":{"ProgressPercent":12.6}}}}`),
		json.RawMessage(`{"ChangedEntitlement":{
			"BookEntitlement":{"Id":"b","Accessibility":"Full","OriginCategory":"KoboPlus"},
			"BookMetadata":{"Title":"B","ContributorRoles":[
				{"Name":"Y","Role":"Author"},{"Name":"Z","Role":"Translator"}]}}}`),
		json.RawMessage(`{"ChangedReadingState":{"ReadingState":{"EntitlementId":"c",
			"StatusInfo":{"Status":"Finished"}}}}`),
		json.RawMessage(`{"ChangedReadingState":{"ReadingState":{}}}`),
		json.RawMessage(`{"NewTag":{}}`),
		json.RawMessage(`not json`),
	}

	storeBooks, readings := parseKoboStoreItems(items)

	require.Len(t, storeBooks, 2)
	assert.Equal(t, "a", storeBooks[0].EntitlementID)
	assert.Equal(t, "9780000000001", *storeBooks[0].ISBN13)
	assert.Equal(t, []string{"X"}, storeBooks[0].Authors)
	assert.True(t, *storeBooks[0].Owned)
	assert.Nil(t, storeBooks[1].ISBN13)
	assert.Equal(t, []string{"Y"}, storeBooks[1].Authors)
	assert.False(t, *storeBooks[1].Owned)

	require.Len(t, readings, 2)
	assert.Equal(t, "a", readings[0].EntitlementID)
	assert.Equal(t, 13, readings[0].Percent)
	assert.Equal(t, "c", readings[1].EntitlementID)
	assert.True(t, readings[1].Finished)
}

func TestKoboStorePercent(t *testing.T) {
	for in, want := range map[float64]int{
		0: 0, 0.01: 1, 0.49: 1, 0.99: 1, 1.4: 1, 12.6: 13, 100: 100,
	} {
		assert.Equal(t, want, koboStorePercent(in), "%v", in)
	}
}

func TestKoboStoreOwned(t *testing.T) {
	owned := func(ent string) bool {
		var e koboStoreEntry
		require.NoError(t, json.Unmarshal([]byte(`{"BookEntitlement":`+ent+`}`), &e))
		return koboStoreOwned(&e)
	}
	assert.True(t, owned(`{"Accessibility":"Full","ActivePeriod":{"From":"x"}}`))
	assert.False(t, owned(`{"Accessibility":"Preview"}`))
	assert.False(t, owned(`{"Accessibility":"Full","IsRemoved":true}`))
	assert.False(t, owned(`{"Accessibility":"Full","ActivePeriod":{"To":"x"}}`))
	assert.False(t, owned(`{"Accessibility":"Full","OriginCategory":"Subscription"}`))
}
