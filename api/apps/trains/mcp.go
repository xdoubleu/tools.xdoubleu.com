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

// mcpSearchStationsArgs matches a substring of a station's name in any of
// Dutch, French or English; an empty query pages through all stations.
type mcpSearchStationsArgs struct {
	Query string `json:"query,omitempty" jsonschema:"name substring (nl|fr|en)"`
}

// mcpSearchJourneysArgs takes stop ids as trains_search_stations reports
// them. ArriveBy reads Time as an arrival deadline instead of a departure.
type mcpSearchJourneysArgs struct {
	OriginStopID      string `json:"origin_stop_id"      jsonschema:"origin stop id"`
	DestinationStopID string `json:"destination_stop_id" jsonschema:"destination stop id"`
	Time              string `json:"time,omitempty"      jsonschema:"RFC3339, empty=now"`
	ArriveBy          bool   `json:"arrive_by,omitempty" jsonschema:"time is a deadline"`
}

// RegisterMCPTools exposes the trains app's read-only RPCs on the combined
// apps MCP server. The timetable is public data shared by every user, so
// unlike the other apps' tools these return the same rows for any caller
// holding trains access.
func (a *Trains) RegisterMCPTools(srv *mcp.Server) {
	h := &trainsConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "trains_search_stations",
		"Stations matching a query, each with its Dutch, French and English "+
			"name — the names the /trains pickers search and display.",
		h.mcpSearchStations)
	mcptools.AddReadTool(srv, mcpAppName, "trains_get_feed_info",
		"The imported SNCB/NMBS timetable's feed version and when the import "+
			"that produced it ran. A conditional GET makes an unchanged feed a "+
			"no-op, so a stale imported_at is how a skipped import shows up.",
		h.mcpGetFeedInfo)
	mcptools.AddReadTool(srv, mcpAppName, "trains_search_journeys",
		"Journeys between two stops around a time, as the /trains route "+
			"overview computes them.", h.mcpSearchJourneys)
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
