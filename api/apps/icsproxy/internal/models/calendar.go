package models

type FilterConfig struct {
	Token          string
	UserID         string
	SourceURL      string
	HideEventUIDs  []string
	HolidayUIDs    []string
	HideSeries     map[string]bool // SeriesKey -> hide
	HideUnaccepted bool
}

type EventInfo struct {
	UID             string
	Summary         string
	StartRaw        string
	EndRaw          string
	StartNice       string
	EndNice         string
	RRule           string
	SeriesKey       string
	HasRecurrenceID bool
}
