package models_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"tools.xdoubleu.com/apps/goaltracker/internal/models"
)

func TestTodoistDueStringToPeriod(t *testing.T) {
	var nilPtr *int
	assert.Equal(t, nilPtr, models.TodoistDueStringToPeriod(""))
	assert.Equal(t, models.Year, *models.TodoistDueStringToPeriod("every year"))
	assert.Equal(t, models.Quarter, *models.TodoistDueStringToPeriod("every 3 months"))
}

func TestAdaptiveTargetValues(t *testing.T) {
	val := int64(365)
	val2 := models.Year
	val3 := time.Date(time.Now().Year(), 12, 31, 0, 0, 0, 0, time.UTC)

	//nolint:exhaustruct //I know
	goal := models.Goal{
		TargetValue: &val,
		Period:      &val2,
		DueTime:     &val3,
	}

	goalValues := goal.AdaptiveTargetValues(1)
	for i := 0; i < 365; i++ {
		assert.Equal(t, fmt.Sprintf("%.2f", float64(i+1)), goalValues[i])
	}
}
