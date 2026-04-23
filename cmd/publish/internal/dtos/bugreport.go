package dtos

type BugReportDto struct {
	Title       string `schema:"title"`
	Description string `schema:"description"`
	Page        string `schema:"page"`
}
