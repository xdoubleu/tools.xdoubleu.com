package services

// Services are the app's use cases, shared by its adapters.
type Services struct {
	Scope    *ScopeService
	Reads    *ReadService
	Shows    *ShowService
	Profiles *ProfileService
	Picks    *RecommendService
	Notify   *NotifyService
}
