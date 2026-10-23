package irail

// iRail encodes every number as a JSON string, so the wire structs are all
// strings and the parse functions convert.

type wireStationInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type wireVehicleInfo struct {
	Type   string `json:"type"`
	Number string `json:"number"`
}

type wirePlatformInfo struct {
	Name string `json:"name"`
}

type wireEndpoint struct {
	Delay       string           `json:"delay"`
	Station     string           `json:"station"`
	StationInfo wireStationInfo  `json:"stationinfo"`
	Time        string           `json:"time"`
	Vehicle     string           `json:"vehicle"`
	VehicleInfo wireVehicleInfo  `json:"vehicleinfo"`
	Platform    string           `json:"platform"`
	PlatInfo    wirePlatformInfo `json:"platforminfo"`
	Canceled    string           `json:"canceled"`
	Walking     string           `json:"walking"`
	Direction   struct {
		Name string `json:"name"`
	} `json:"direction"`
}

type wireVia struct {
	Arrival   wireEndpoint `json:"arrival"`
	Departure wireEndpoint `json:"departure"`
}

type wireConnection struct {
	Departure wireEndpoint `json:"departure"`
	Arrival   wireEndpoint `json:"arrival"`
	Vias      struct {
		Via []wireVia `json:"via"`
	} `json:"vias"`
	Alerts struct {
		Alert []struct {
			Header      string `json:"header"`
			Description string `json:"description"`
		} `json:"alert"`
	} `json:"alerts"`
}

type wireConnections struct {
	Connection []wireConnection `json:"connection"`
}

func (e wireEndpoint) endpoint() Endpoint {
	platform := e.Platform
	if platform == "" {
		platform = e.PlatInfo.Name
	}
	return Endpoint{
		StationID: StationUIC(e.StationInfo.ID),
		Name:      e.StationInfo.Name,
		Time:      unixTime(e.Time),
		Delay:     atoi(e.Delay),
		Platform:  cleanPlatform(platform),
		Canceled:  e.Canceled == "1",
	}
}

func parseConnections(body []byte) ([]Connection, error) {
	var w wireConnections
	if err := decode(body, &w); err != nil {
		return nil, err
	}
	out := make([]Connection, 0, len(w.Connection))
	for _, wc := range w.Connection {
		out = append(out, wc.connection())
	}
	return out, nil
}

// connection splits a connection into legs. A walking transfer has no vehicle
// and is dropped, so a leg is always one boarded train.
func (wc wireConnection) connection() Connection {
	boards := []wireEndpoint{wc.Departure}
	alights := make([]wireEndpoint, 0, len(wc.Vias.Via)+1)
	for _, via := range wc.Vias.Via {
		alights = append(alights, via.Arrival)
		boards = append(boards, via.Departure)
	}
	alights = append(alights, wc.Arrival)

	var legs []Leg
	for i, board := range boards {
		if board.Walking == "1" || board.Vehicle == "" {
			continue
		}
		legs = append(legs, Leg{
			VehicleID: board.Vehicle,
			Type:      board.VehicleInfo.Type,
			Number:    board.VehicleInfo.Number,
			Direction: board.Direction.Name,
			Board:     board.endpoint(),
			Alight:    alights[i].endpoint(),
		})
	}

	alerts := make([]Alert, len(wc.Alerts.Alert))
	for i, a := range wc.Alerts.Alert {
		alerts[i] = Alert{Header: a.Header, Description: a.Description}
	}
	return Connection{Legs: legs, Alerts: alerts}
}

type wireStations struct {
	Station []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"station"`
}

func parseStations(body []byte) ([]Station, error) {
	var w wireStations
	if err := decode(body, &w); err != nil {
		return nil, err
	}
	out := make([]Station, 0, len(w.Station))
	for _, s := range w.Station {
		out = append(out, Station{ID: StationUIC(s.ID), Name: s.Name})
	}
	return out, nil
}

type wireVehicle struct {
	Vehicle     string          `json:"vehicle"`
	VehicleInfo wireVehicleInfo `json:"vehicleinfo"`
	Stops       struct {
		Stop []struct {
			StationInfo        wireStationInfo  `json:"stationinfo"`
			Platform           string           `json:"platform"`
			PlatInfo           wirePlatformInfo `json:"platforminfo"`
			ScheduledArrival   string           `json:"scheduledArrivalTime"`
			ScheduledDeparture string           `json:"scheduledDepartureTime"`
			ArrivalDelay       string           `json:"arrivalDelay"`
			DepartureDelay     string           `json:"departureDelay"`
			ArrivalCanceled    string           `json:"arrivalCanceled"`
			DepartureCanceled  string           `json:"departureCanceled"`
		} `json:"stop"`
	} `json:"stops"`
}

func parseVehicle(body []byte) (*Vehicle, error) {
	var w wireVehicle
	if err := decode(body, &w); err != nil {
		return nil, err
	}
	v := &Vehicle{
		ID:    w.Vehicle,
		Type:  w.VehicleInfo.Type,
		Num:   w.VehicleInfo.Number,
		Calls: make([]Call, 0, len(w.Stops.Stop)),
	}
	for _, s := range w.Stops.Stop {
		platform := s.Platform
		if platform == "" {
			platform = s.PlatInfo.Name
		}
		v.Calls = append(v.Calls, Call{
			StationID:          StationUIC(s.StationInfo.ID),
			Name:               s.StationInfo.Name,
			Platform:           cleanPlatform(platform),
			ScheduledArrival:   unixTime(s.ScheduledArrival),
			ScheduledDeparture: unixTime(s.ScheduledDeparture),
			ArrivalDelay:       atoi(s.ArrivalDelay),
			DepartureDelay:     atoi(s.DepartureDelay),
			ArrivalCanceled:    s.ArrivalCanceled == "1",
			DepartureCanceled:  s.DepartureCanceled == "1",
		})
	}
	return v, nil
}

// cleanPlatform drops iRail's "?" placeholder for an unknown platform.
func cleanPlatform(p string) string {
	if p == "?" {
		return ""
	}
	return p
}
