package trains

import (
	"context"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

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

// RegisterMCPTools exposes the trains read RPCs on the apps MCP server. The
// timetable is public, so every caller with trains access gets the same rows.
func (a *Trains) RegisterMCPTools(srv *mcp.Server) {
	h := &trainsConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "trains_search_stations",
		"Stations matching a query, each with its Dutch, French and English "+
			"name — the names the /trains pickers search and display.",
		h.mcpSearchStations)
	mcptools.AddReadTool(srv, mcpAppName, "trains_get_feed_info",
		"The imported SNCB/NMBS timetable's feed version, when the import "+
			"that produced it ran, and how much of the feed's translations.txt "+
			"it applied. A conditional GET makes an unchanged feed a no-op, so "+
			"a stale imported_at is how a skipped import shows up; translated "+
			"stop counts of 0 against a non-zero rows count is how station "+
			"names silently staying monolingual shows up.",
		h.mcpGetFeedInfo)
	mcptools.AddReadTool(srv, mcpAppName, "trains_search_journeys",
		"Journeys between two stops around a time, as the /trains route "+
			"overview computes them.", h.mcpSearchJourneys)
	mcptools.AddReadTool(srv, mcpAppName, "trains_get_journey_detail",
		"The full live state of one previously-searched journey — every stop "+
			"of every boarded train with its scheduled time, live delay/"+
			"cancellation status and platform, plus attached service alerts. "+
			"This is what /trains/[journeyId] renders. The realtime feed is "+
			"replaced wholesale every 30s and nothing is retained, so this "+
			"only ever reports the current state, never what it looked like "+
			"earlier.",
		h.mcpGetJourneyDetail)
}

func (h *trainsConnectHandler) mcpSearchStations(
	ctx context.Context, args mcpSearchStationsArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.SearchStations(ctx, connect.NewRequest(
		&trainsv1.SearchStationsRequest{Query: args.Query},
	)))
}

func (h *trainsConnectHandler) mcpGetFeedInfo(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetFeedInfo(ctx, connect.NewRequest(
		&trainsv1.GetFeedInfoRequest{},
	)))
}

func (h *trainsConnectHandler) mcpSearchJourneys(
	ctx context.Context, args mcpSearchJourneysArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.SearchJourneys(ctx, connect.NewRequest(
		&trainsv1.SearchJourneysRequest{
			OriginStopId:      args.OriginStopID,
			DestinationStopId: args.DestinationStopID,
			Time:              args.Time,
			ArriveBy:          args.ArriveBy,
		},
	)))
}

func (h *trainsConnectHandler) mcpGetJourneyDetail(
	ctx context.Context, args mcpGetJourneyDetailArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetJourneyDetail(ctx, connect.NewRequest(
		&trainsv1.GetJourneyDetailRequest{JourneyId: args.JourneyID},
	)))
}
