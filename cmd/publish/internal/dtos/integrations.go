package dtos

import (
	"regexp"

	"github.com/xdoubleu/essentia/v4/pkg/validate"
)

type IntegrationsDto struct {
	SteamAPIKey     string `schema:"steam_api_key"`
	SteamUserID     string `schema:"steam_user_id"`
	HardcoverAPIKey string `schema:"hardcover_api_key"`
}

const (
	steamAPIKeyMaxLen     = 64
	steamUserIDMaxLen     = 20
	hardcoverAPIKeyMaxLen = 256
)

func (dto *IntegrationsDto) Validate() (bool, map[string]string) {
	v := validate.New()

	validate.Check(v, "steam_api_key", dto.SteamAPIKey, maxLen(steamAPIKeyMaxLen))
	validate.Check(v, "steam_user_id", dto.SteamUserID, isNumericOrEmpty)
	validate.Check(v, "steam_user_id", dto.SteamUserID, maxLen(steamUserIDMaxLen))
	validate.Check(
		v,
		"hardcover_api_key",
		dto.HardcoverAPIKey,
		maxLen(hardcoverAPIKeyMaxLen),
	)

	return v.Valid(), v.Errors()
}

var numericRe = regexp.MustCompile(`^\d*$`)

func isNumericOrEmpty(s string) (bool, string) {
	if !numericRe.MatchString(s) {
		return false, "must contain digits only"
	}
	return true, ""
}

func maxLen(n int) validate.ValidatorFunc[string] {
	return func(s string) (bool, string) {
		if len(s) > n {
			return false, "too long"
		}
		return true, ""
	}
}
