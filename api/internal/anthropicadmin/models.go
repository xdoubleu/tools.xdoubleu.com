package anthropicadmin

// UsageRecord is one row of the Claude Code Analytics usage report: one
// actor's (user or API key) Claude Code activity for a single UTC day,
// broken down by model. Field shape confirmed against the "Get Claude Code
// Usage Report" page of Anthropic's API reference
// (platform.claude.com/docs, under api/beta/organization/usage_report/
// retrieve_claude_code).
type UsageRecord struct {
	// CustomerType is "api" (metered API-key billing) or "subscription"
	// (Pro/Max/Team) — the Analytics API is the one endpoint that reports
	// both, unlike usage_report/messages and cost_report which only cover
	// metered API-key billing.
	CustomerType string `json:"customer_type"`
	// Date is the UTC day these metrics cover, RFC 3339 at midnight UTC.
	Date string `json:"date"`
	// ModelBreakdown holds token usage and estimated cost per model used
	// by this actor on this day.
	ModelBreakdown []ModelBreakdown `json:"model_breakdown"`
}

// ModelBreakdown is one model's token usage and estimated cost within a
// UsageRecord.
type ModelBreakdown struct {
	Model         string        `json:"model"`
	Tokens        Tokens        `json:"tokens"`
	EstimatedCost EstimatedCost `json:"estimated_cost"`
}

// Tokens is the per-model token usage breakdown.
type Tokens struct {
	Input         float64 `json:"input"`
	Output        float64 `json:"output"`
	CacheCreation float64 `json:"cache_creation"`
	CacheRead     float64 `json:"cache_read"`
}

// EstimatedCost is the per-model estimated cost, in minor currency units
// (e.g. cents for USD).
type EstimatedCost struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// usageReportWire is the raw JSON envelope GET
// /v1/organizations/usage_report/claude_code returns — a cursor-paginated
// page of UsageRecord plus the fields needed to fetch the next one.
type usageReportWire struct {
	Data     []UsageRecord `json:"data"`
	HasMore  bool          `json:"has_more"`
	NextPage *string       `json:"next_page"`
}
