// Package services holds the trains app's business logic.
package services

import (
	"tools.xdoubleu.com/apps/trains/pkg/irail"
)

type Services struct {
	Journey       *JourneyService
	Stations      *StationsService
	JourneyDetail *JourneyDetailService
	SavedCommutes *SavedCommutesService
	Irail         *IrailService
	// JourneyWatch is set once the push topics exist (see WatchJourneys),
	// because the topics fetch detail through JourneyDetail.
	JourneyWatch *JourneyWatchService
}

func New(client irail.Client, commutes SavedCommuteStore) *Services {
	stations := NewStationsService(client)
	journey := NewJourneyService(client, stations)

	//nolint:exhaustruct //JourneyWatch is set by WatchJourneys
	return &Services{
		Journey:       journey,
		Stations:      stations,
		JourneyDetail: NewJourneyDetailService(client, stations, journey),
		SavedCommutes: NewSavedCommutesService(commutes, stations),
		Irail:         NewIrailService(client),
	}
}

// WatchJourneys publishes watched journeys on topics.
func (s *Services) WatchJourneys(topics JourneyTopics) {
	s.JourneyWatch = NewJourneyWatchService(s.JourneyDetail, topics)
}
