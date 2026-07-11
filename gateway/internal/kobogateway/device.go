package kobogateway

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	confRelPath    = ".kobo/Kobo/Kobo eReader.conf"
	versionRelPath = ".kobo/version"
)

// Kobo describes a mounted Kobo volume as reported by /status.
type Kobo struct {
	VolumePath      string `json:"volumePath"`
	Serial          string `json:"serial"`
	CurrentEndpoint string `json:"currentEndpoint"`
}

// FindKobos scans the direct children of volumesRoot (normally /Volumes) and
// returns every mount that contains a Kobo eReader.conf. Volumes that cannot
// be read are skipped rather than failing the whole scan.
func FindKobos(volumesRoot string) ([]Kobo, error) {
	entries, err := os.ReadDir(volumesRoot)
	if err != nil {
		return nil, fmt.Errorf("could not read volumes root: %w", err)
	}

	kobos := []Kobo{}
	for _, entry := range entries {
		volumePath := filepath.Join(volumesRoot, entry.Name())

		conf, confErr := readConfFile(volumePath)
		if confErr != nil {
			continue
		}

		kobos = append(kobos, Kobo{
			VolumePath:      volumePath,
			Serial:          ReadSerial(volumePath),
			CurrentEndpoint: conf.APIEndpoint(),
		})
	}

	return kobos, nil
}

// ReadSerial mirrors readKoboSerial in web/lib/books/koboDevice.ts: the
// .kobo/version file is a comma-separated line whose first field is the
// serial number. Returns an empty string when unavailable.
func ReadSerial(volumePath string) string {
	raw, err := os.ReadFile(filepath.Join(volumePath, versionRelPath))
	if err != nil {
		return ""
	}

	serial, _, _ := strings.Cut(strings.TrimSpace(string(raw)), ",")

	return strings.TrimSpace(serial)
}

func confPath(volumePath string) string {
	return filepath.Join(volumePath, confRelPath)
}

func readConfFile(volumePath string) (*Conf, error) {
	raw, err := os.ReadFile(confPath(volumePath))
	if err != nil {
		return nil, fmt.Errorf("could not read Kobo eReader.conf: %w", err)
	}

	return ParseConf(string(raw)), nil
}

func writeConfFile(volumePath string, conf *Conf) error {
	path := confPath(volumePath)

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("could not stat Kobo eReader.conf: %w", err)
	}

	err = os.WriteFile(path, []byte(conf.Serialize()), info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("could not write Kobo eReader.conf: %w", err)
	}

	return nil
}
