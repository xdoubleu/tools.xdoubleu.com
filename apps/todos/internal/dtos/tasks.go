package dtos

type QuickAddDto struct {
	Input     string `schema:"input"`
	SectionID string `schema:"section_id"`
}

type SaveTaskDto struct {
	Title       string   `schema:"title"`
	Description string   `schema:"description"`
	Label       string   `schema:"label"`
	DueDate     string   `schema:"due_date"`
	Deadline    string   `schema:"deadline"`
	SectionID   string   `schema:"section_id"`
	Priority    int      `schema:"priority"`
	RecurDays   int      `schema:"recur_days"`
	LinkURLs    []string `schema:"link_url"`
	LinkLabels  []string `schema:"link_label"`
}

type ReorderDto struct {
	IDs []string `json:"ids"`
}

type AddLabelPresetDto struct {
	Category string `schema:"category"`
	Value    string `schema:"value"`
}

type AddURLPatternDto struct {
	URLPrefix    string `schema:"url_prefix"`
	PlatformName string `schema:"platform_name"`
	Label        string `schema:"label"`
	Shortcut     string `schema:"shortcut"`
}

type UpdateArchiveDto struct {
	ArchiveAfterHours int `schema:"archive_after_hours"`
}

type AddSubtaskDto struct {
	Title string `schema:"title"`
}

type AddSectionDto struct {
	Name string `schema:"name"`
}

type AddPolicyDto struct {
	Text               string `schema:"text"`
	ReappearAfterHours int    `schema:"reappear_after_hours"`
}

type AddWorkspaceDto struct {
	Name string `schema:"name"`
}

type SetModeDto struct {
	WorkspaceID string `schema:"workspace_id"`
	Back        string `schema:"back"`
}
