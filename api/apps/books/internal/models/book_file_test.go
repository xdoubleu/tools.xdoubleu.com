package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/books/internal/models"
)

func TestLibraryFiles_ReadSuggestion(t *testing.T) {
	pdf, epub := models.FileFormatPDF, models.FileFormatEPUB
	tests := []struct {
		name  string
		files models.LibraryFiles
		want  *models.ReadSuggestion
	}{
		{
			name: "figure-heavy pdf",
			files: models.LibraryFiles{ //nolint:exhaustruct // no versions needed
				Formats: []string{pdf},
				Figures: map[string]models.FigureStats{pdf: {Figures: 15, Pages: 100}},
			},
			want: &models.ReadSuggestion{Format: pdf, FigureCount: 15, Pages: 100},
		},
		{
			name: "just under the threshold",
			files: models.LibraryFiles{ //nolint:exhaustruct // no versions needed
				Formats: []string{pdf},
				Figures: map[string]models.FigureStats{pdf: {Figures: 14, Pages: 100}},
			},
			want: &models.ReadSuggestion{Format: epub, FigureCount: 14, Pages: 100},
		},
		{
			name: "pdf decides over epub",
			files: models.LibraryFiles{ //nolint:exhaustruct // no versions needed
				Formats: []string{epub, pdf},
				Figures: map[string]models.FigureStats{
					pdf:  {Figures: 0, Pages: 300},
					epub: {Figures: 90, Pages: 200},
				},
			},
			want: &models.ReadSuggestion{Format: epub, FigureCount: 0, Pages: 300},
		},
		{
			name: "pdf not analysed yet",
			files: models.LibraryFiles{ //nolint:exhaustruct // no versions needed
				Formats: []string{epub, pdf},
				Figures: map[string]models.FigureStats{epub: {Figures: 90, Pages: 200}},
			},
			want: nil,
		},
		{
			name: "epub only",
			files: models.LibraryFiles{ //nolint:exhaustruct // no versions needed
				Formats: []string{epub},
				Figures: map[string]models.FigureStats{epub: {Figures: 90, Pages: 200}},
			},
			want: &models.ReadSuggestion{Format: pdf, FigureCount: 90, Pages: 200},
		},
		{
			name: "unanalysable file",
			files: models.LibraryFiles{ //nolint:exhaustruct // no versions needed
				Formats: []string{epub},
				Figures: map[string]models.FigureStats{epub: {Figures: 0, Pages: 0}},
			},
			want: nil,
		},
		{
			name:  "no files",
			files: models.LibraryFiles{}, //nolint:exhaustruct // empty on purpose
			want:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.files.ReadSuggestion())
		})
	}
}
