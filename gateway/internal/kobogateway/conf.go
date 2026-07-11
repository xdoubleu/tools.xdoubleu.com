// Package kobogateway implements the local macOS gateway that configures a
// USB-mounted Kobo e-reader for sync with this server. It mirrors the browser
// flow in web/components/books/KoboSetup.tsx: the web UI performs all
// authenticated API calls and hands only the resulting sync URL to this
// gateway, which does the local file work.
package kobogateway

import "strings"

const (
	targetSection = "OneStoreServices"
	targetKey     = "api_endpoint"

	// DefaultKoboEndpoint is the stock Kobo store endpoint that ships with
	// every Kobo device.
	DefaultKoboEndpoint = "https://storeapi.kobo.com"
)

// KV is a single key=value entry inside a conf section.
type KV struct {
	Key   string
	Value string
}

// Section is a named [Section] block of a Kobo eReader.conf file.
type Section struct {
	Name string
	Keys []KV
}

// Conf is an ordered representation of a Kobo eReader.conf file. Order is
// preserved so that a device configured by the browser (which relies on JS
// object insertion order) round-trips identically through this gateway.
type Conf struct {
	Sections []Section
}

// ParseConf mirrors parseKoboConf in web/lib/books/koboConf.ts: only
// [section] headers and key=value lines are kept, everything else is
// dropped. Repeated sections merge into the first occurrence and repeated
// keys overwrite in place.
func ParseConf(raw string) *Conf {
	conf := &Conf{Sections: nil}
	sectionIdx := -1

	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))

		switch {
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			name := line[1 : len(line)-1]
			sectionIdx = conf.sectionIndex(name)
			if sectionIdx < 0 {
				conf.Sections = append(
					conf.Sections,
					Section{Name: name, Keys: nil},
				)
				sectionIdx = len(conf.Sections) - 1
			}
		case sectionIdx >= 0 && strings.Contains(line, "="):
			key, value, _ := strings.Cut(line, "=")
			setKey(&conf.Sections[sectionIdx], key, value)
		}
	}

	return conf
}

// Serialize mirrors serializeKoboConf in web/lib/books/koboConf.ts: sections
// are joined by a blank line and the file carries no trailing newline.
func (c *Conf) Serialize() string {
	sections := make([]string, 0, len(c.Sections))
	for _, section := range c.Sections {
		lines := make([]string, 0, len(section.Keys)+1)
		lines = append(lines, "["+section.Name+"]")
		for _, kv := range section.Keys {
			lines = append(lines, kv.Key+"="+kv.Value)
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	return strings.Join(sections, "\n\n")
}

// APIEndpoint returns the current [OneStoreServices] api_endpoint value, or
// an empty string when it is not set.
func (c *Conf) APIEndpoint() string {
	idx := c.sectionIndex(targetSection)
	if idx < 0 {
		return ""
	}

	for _, kv := range c.Sections[idx].Keys {
		if kv.Key == targetKey {
			return kv.Value
		}
	}

	return ""
}

// SetAPIEndpoint sets [OneStoreServices] api_endpoint (creating the section
// when missing) and returns the previous value, mirroring patchApiEndpoint /
// revertApiEndpoint in web/lib/books/koboConf.ts.
func (c *Conf) SetAPIEndpoint(endpoint string) string {
	original := c.APIEndpoint()

	idx := c.sectionIndex(targetSection)
	if idx < 0 {
		c.Sections = append(
			c.Sections,
			Section{Name: targetSection, Keys: nil},
		)
		idx = len(c.Sections) - 1
	}
	setKey(&c.Sections[idx], targetKey, endpoint)

	return original
}

func (c *Conf) sectionIndex(name string) int {
	for i, section := range c.Sections {
		if section.Name == name {
			return i
		}
	}

	return -1
}

func setKey(section *Section, key, value string) {
	for i, kv := range section.Keys {
		if kv.Key == key {
			section.Keys[i].Value = value

			return
		}
	}

	section.Keys = append(section.Keys, KV{Key: key, Value: value})
}
