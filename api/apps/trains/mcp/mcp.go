// Package mcp exposes the trains read RPCs as MCP tools.
package mcp

import (
	"context"
	"encoding/json"

	"connectrpc.com/connect/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"tools.xdoubleu.com/apps/trains/internal/protomap"
	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/apps/trains/pkg/irail"
	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/platform/mcptools"
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

type tools struct {
	svc *services.Services
}

// Register exposes the trains read RPCs on the apps MCP server. The timetable
// is public, so every caller with trains access gets the same rows.
func Register(srv *mcp.Server, svc *services.Services) {
	h := &tools{svc: svc}

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
		h.mcpIrailDiagnostic)
}

func mapError(err error) error {
	return connecttools.MapError(
		err,
		connecttools.Code(services.ErrUnknownStation, connect.CodeNotFound),
		connecttools.Code(irail.ErrRateLimited, connect.CodeUnavailable),
		connecttools.Code(services.ErrInvalidJourneyID, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrJourneyIDRequired, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrInvalidTime, connect.CodeInvalidArgument),
	)
}

func (h *tools) mcpSearchStations(
	ctx context.Context, args mcpSearchStationsArgs,
) (proto.Message, error) {
	stations, err := h.svc.Stations.SearchStations(ctx, args.Query)
	if err != nil {
		return nil, mapError(err)
	}
	return &trainsv1.SearchStationsResponse{
		Stations: protomap.Stations(stations),
	}, nil
}

func (h *tools) mcpSearchJourneys(
	ctx context.Context, args mcpSearchJourneysArgs,
) (proto.Message, error) {
	when, err := services.ParseSearchTime(args.Time)
	if err != nil {
		return nil, mapError(err)
	}
	journeys, err := h.svc.Journey.SearchJourneys(
		ctx, args.OriginStopID, args.DestinationStopID, when, args.ArriveBy,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &trainsv1.SearchJourneysResponse{
		Journeys: protomap.Journeys(journeys),
	}, nil
}

func (h *tools) mcpGetJourneyDetail(
	ctx context.Context, args mcpGetJourneyDetailArgs,
) (proto.Message, error) {
	detail, err := h.svc.JourneyWatch.Watch(ctx, args.JourneyID)
	if err != nil {
		return nil, mapError(err)
	}
	return &trainsv1.GetJourneyDetailResponse{
		Journey: protomap.JourneyDetail(args.JourneyID, detail),
	}, nil
}

func (h *tools) mcpIrailDiagnostic(
	ctx context.Context, args mcpIrailDiagnosticArgs,
) (proto.Message, error) {
	diag, err := h.svc.Irail.Diagnostic(ctx, args.VehicleID, args.Date)
	if err != nil {
		return nil, err
	}

	// Round-trip through JSON so structs become the plain maps structpb needs.
	data, err := json.Marshal(diag)
	if err != nil {
		return nil, err
	}
	var plain map[string]any
	if err = json.Unmarshal(data, &plain); err != nil {
		return nil, err
	}
	return structpb.NewStruct(plain)
}
