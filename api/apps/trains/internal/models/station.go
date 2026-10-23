package models

// Station is a station a passenger can pick. StopID is the 9-digit UIC code
// ("008812005").
type Station struct {
	StopID      string
	NameNL      string
	NameFR      string
	NameEN      string
	DisplayName string
}
