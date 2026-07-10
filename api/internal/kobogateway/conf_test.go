package kobogateway_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/kobogateway"
)

const sampleConf = `[OneStoreServices]
api_endpoint=https://storeapi.kobo.com
affiliate=Kobo

[Version]
BuildVersion=4.37.21586
FirmwareVersion=4.37.21586`

func TestParseConf(t *testing.T) {
	conf := kobogateway.ParseConf(sampleConf)

	assert.Equal(t, "https://storeapi.kobo.com", conf.APIEndpoint())
	assert.Len(t, conf.Sections, 2)
	assert.Equal(t, "OneStoreServices", conf.Sections[0].Name)
	assert.Equal(
		t,
		kobogateway.KV{Key: "affiliate", Value: "Kobo"},
		conf.Sections[0].Keys[1],
	)
	assert.Equal(t, "Version", conf.Sections[1].Name)
	assert.Equal(
		t,
		kobogateway.KV{Key: "BuildVersion", Value: "4.37.21586"},
		conf.Sections[1].Keys[0],
	)
}

func TestParseConfCRLF(t *testing.T) {
	conf := kobogateway.ParseConf(
		"[OneStoreServices]\r\napi_endpoint=https://example.com\r\n",
	)

	assert.Equal(t, "https://example.com", conf.APIEndpoint())
}

func TestParseConfValueWithEquals(t *testing.T) {
	conf := kobogateway.ParseConf("[S]\nkey=https://host/path?a=1&b=2")

	assert.Equal(
		t,
		kobogateway.KV{Key: "key", Value: "https://host/path?a=1&b=2"},
		conf.Sections[0].Keys[0],
	)
}

func TestParseConfEmpty(t *testing.T) {
	conf := kobogateway.ParseConf("")

	assert.Empty(t, conf.Sections)
	assert.Equal(t, "", conf.APIEndpoint())
}

func TestParseConfIgnoresOrphansAndMalformed(t *testing.T) {
	conf := kobogateway.ParseConf("orphan=value\n[S]\nnot-a-pair\nkey=val")

	assert.Len(t, conf.Sections, 1)
	assert.Equal(
		t,
		[]kobogateway.KV{{Key: "key", Value: "val"}},
		conf.Sections[0].Keys,
	)
}

func TestParseConfMergesRepeatedSectionsAndKeys(t *testing.T) {
	conf := kobogateway.ParseConf("[S]\na=1\n[T]\nb=2\n[S]\na=3\nc=4")

	assert.Len(t, conf.Sections, 2)
	assert.Equal(t, []kobogateway.KV{
		{Key: "a", Value: "3"},
		{Key: "c", Value: "4"},
	}, conf.Sections[0].Keys)
}

func TestSerializeRoundTrip(t *testing.T) {
	conf := kobogateway.ParseConf(sampleConf)

	assert.Equal(t, sampleConf, conf.Serialize())
}

func TestSerializeFormat(t *testing.T) {
	conf := kobogateway.ParseConf("[A]\nk=1\n\n[B]\nk=2")

	assert.Equal(t, "[A]\nk=1\n\n[B]\nk=2", conf.Serialize())
}

func TestSetAPIEndpoint(t *testing.T) {
	conf := kobogateway.ParseConf(sampleConf)

	original := conf.SetAPIEndpoint("https://myserver/books/kobo/TOKEN")

	assert.Equal(t, "https://storeapi.kobo.com", original)
	assert.Equal(t, "https://myserver/books/kobo/TOKEN", conf.APIEndpoint())
	// Other keys, sections, and their order are preserved.
	assert.Equal(
		t,
		"[OneStoreServices]\napi_endpoint=https://myserver/books/kobo/TOKEN\n"+
			"affiliate=Kobo\n\n[Version]\nBuildVersion=4.37.21586\n"+
			"FirmwareVersion=4.37.21586",
		conf.Serialize(),
	)
}

func TestSetAPIEndpointCreatesSection(t *testing.T) {
	conf := kobogateway.ParseConf("[Version]\nBuildVersion=1.0")

	original := conf.SetAPIEndpoint("https://myserver/kobo/T")

	assert.Equal(t, "", original)
	assert.Equal(t, "https://myserver/kobo/T", conf.APIEndpoint())
}

func TestSetAPIEndpointRevert(t *testing.T) {
	conf := kobogateway.ParseConf(sampleConf)

	original := conf.SetAPIEndpoint("https://myserver/books/kobo/TOKEN")
	_ = conf.SetAPIEndpoint(original)

	assert.Equal(t, sampleConf, conf.Serialize())
}

func TestDefaultKoboEndpoint(t *testing.T) {
	assert.Equal(
		t,
		"https://storeapi.kobo.com",
		kobogateway.DefaultKoboEndpoint,
	)
}
