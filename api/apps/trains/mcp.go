package trains

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"tools.xdoubleu.com/apps/trains/pkg/irail"
	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
	"tools.xdoubleu.com/internal/mcptools"
)

const mcpAppName = "trains"

// mcpSearchStationsArgs matches a name substring in any language; empty
// pages through all stations.
type mcpSearchStationsArgs struct {
	Query string `json:"query,omitempty" jsonschema:"name substring (nl|fr|en)"`
}

// mcpSearchJourneysArgs takes stop ids from trains_search_stations. ArriveBy
// treats Time as an arrival deadline.
type mcpSearchJourneysArgs struct {
	OriginStopID      string `json:"origin_stop_id"      jsonschema:"origin stop id"`
	DestinationStopID string `json:"destination_stop_id" jsonschema:"destination stop id"`
	Time              string `json:"time,omitempty"      jsonschema:"RFC3339, empty=now"`
	ArriveBy          bool   `json:"arrive_by,omitempty" jsonschema:"time is a deadline"`
}

// mcpGetJourneyDetailArgs takes a journey_id from trains_search_journeys.
type mcpGetJourneyDetailArgs struct {
	JourneyID string `json:"journey_id" jsonschema:"id from trains_search_journeys"`
}

// mcpIrailDiagnosticArgs names a train to fetch raw from iRail.
type mcpIrailDiagnosticArgs struct {
	VehicleID string `json:"vehicle_id,omitempty" jsonschema:"train, e.g. IC530"`
	Date      string `json:"date,omitempty"       jsonschema:"YYYY-MM-DD, empty=today"`
}

// RegisterMCPTools exposes the trains read RPCs on the apps MCP server. The
// timetable is public, so every caller with trains access gets the same rows.
func (a *Trains) RegisterMCPTools(srv *mcp.Server) {
	h := &trainsConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "trains_search_stations",
		"Stations matching a query, each with its Dutch, French and English "+
			"name — the names the /trains pickers search and display.",
		h.mcpSearchStations)
	mcptools.AddReadTool(srv, mcpAppName, "trains_search_journeys",
		"Journeys between two stops around a time, as the /trains route "+
			"overview computes them.", h.mcpSearchJourneys)
	mcptools.AddReadTool(srv, mcpAppName, "trains_get_journey_detail",
		"The full live state of one previously-searched journey — every stop "+
			"of every boarded train with its scheduled time, live delay/"+
			"cancellation status and platform. This is what "+
			"/trains/[journeyId] renders. Nothing is retained, so this only "+
			"ever reports the current state, never what it looked like "+
			"earlier.",
		h.mcpGetJourneyDetail)
	mcptools.AddReadTool(srv, mcpAppName, "trains_get_irail_diagnostic",
		"iRail request health: total requests, 429s, failures and the most "+
			"recent errors since the process started. With a vehicle_id "+
			"(e.g. IC530), also iRail's raw response for that train today — "+
			"how a wrong or missing delay on /trains is traced to iRail "+
			"versus our own mapping.",
		a.mcpIrailDiagnostic)
}

func (h *trainsConnectHandler) mcpSearchStations(
	ctx context.Context, args mcpSearchStationsArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(
		h.SearchStations(ctx, &trainsv1.SearchStationsRequest{Query: args.Query}),
	)
}

func (h *trainsConnectHandler) mcpSearchJourneys(
	ctx context.Context, args mcpSearchJourneysArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.SearchJourneys(ctx, &trainsv1.SearchJourneysRequest{
		OriginStopId:      args.OriginStopID,
		DestinationStopId: args.DestinationStopID,
		Time:              args.Time,
		ArriveBy:          args.ArriveBy,
	}))
}

func (h *trainsConnectHandler) mcpGetJourneyDetail(
	ctx context.Context, args mcpGetJourneyDetailArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(
		h.GetJourneyDetail(ctx, &trainsv1.GetJourneyDetailRequest{JourneyId: args.JourneyID}),
	)
}

func (a *Trains) mcpIrailDiagnostic(
	ctx context.Context, args mcpIrailDiagnosticArgs,
) (proto.Message, error) {
	out := map[string]any{"diagnostics": a.irail.Diagnostics()}

	if args.VehicleID != "" {
		vehicle, err := a.rawVehicle(ctx, args)
		if err != nil {
			return nil, err
		}
		out["vehicle"] = vehicle
	}

	// Round-trip through JSON so structs become the plain maps structpb needs.
	data, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	var plain map[string]any
	if err = json.Unmarshal(data, &plain); err != nil {
		return nil, err
	}
	return structpb.NewStruct(plain)
}

// rawVehicle fetches one train's unparsed iRail response; an iRail failure is
// reported in the result rather than failing the whole diagnostic.
func (a *Trains) rawVehicle(
	ctx context.Context, args mcpIrailDiagnosticArgs,
) (map[string]any, error) {
	day := time.Now()
	if args.Date != "" {
		parsed, err := time.Parse(time.DateOnly, args.Date)
		if err != nil {
			return nil, fmt.Errorf("date must be YYYY-MM-DD: %w", err)
		}
		day = parsed
	}

	id := irail.VehicleID(args.VehicleID)
	vehicle := map[string]any{"id": id}
	raw, err := a.irail.RawVehicle(ctx, id, day)
	if err != nil {
		vehicle["error"] = err.Error()
		return vehicle, nil //nolint:nilerr //the failure is the diagnostic
	}
	var decoded any
	if err = json.Unmarshal(raw, &decoded); err != nil {
		vehicle["error"] = err.Error()
		return vehicle, nil //nolint:nilerr //the failure is the diagnostic
	}
	vehicle["raw"] = decoded
	return vehicle, nil
}
