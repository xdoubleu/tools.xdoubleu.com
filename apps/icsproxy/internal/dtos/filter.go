package dtos

import (
	"net/url"
	"strings"

	"github.com/xdoubleu/essentia/v3/pkg/validate"
)

type PreviewDto struct {
	SourceURL string `schema:"source_url"`
}

func (dto *PreviewDto) Validate() (bool, map[string]string) {
	v := validate.New()
	validate.Check(v, "source_url", dto.SourceURL, validate.IsNotEmpty)
	return v.Valid(), v.Errors()
}

type CreateFilterDto struct {
	SourceURL     string   `schema:"source_url"`
	Token         string   `schema:"token"`
	HideEventUIDs []string `schema:"hide_uid"`
	HolidayUIDs   []string `schema:"holiday_uid"`
}

func (dto *CreateFilterDto) Validate() (bool, map[string]string) {
	v := validate.New()
	validate.Check(v, "source_url", dto.SourceURL, validate.IsNotEmpty)
	return v.Valid(), v.Errors()
}

// HideSeries extracts dynamic hide_rec_* keys from the raw form values.
func (dto *CreateFilterDto) HideSeries(form url.Values) map[string]bool {
	result := map[string]bool{}
	for key := range form {
		if strings.HasPrefix(key, "hide_rec_") {
			result[strings.TrimPrefix(key, "hide_rec_")] = true
		}
	}
	return result
}
