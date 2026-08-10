package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExportWindow_BeforeBreakfastEnd(t *testing.T) {
	now := time.Date(2026, 1, 1, slotBreakfastEnd-1, 0, 0, 0, time.UTC)
	today, end, pastSlots := ExportWindow(now)
	assert.Equal(t, now.Truncate(hoursPerDay*time.Hour), today)
	assert.Equal(t, today.AddDate(0, 0, daysPerWeek-1), end)
	assert.Empty(t, pastSlots)
}

func TestExportWindow_AtBreakfastEnd(t *testing.T) {
	now := time.Date(2026, 1, 1, slotBreakfastEnd, 0, 0, 0, time.UTC)
	today, end, pastSlots := ExportWindow(now)
	assert.Equal(t, now.Truncate(hoursPerDay*time.Hour), today)
	assert.Equal(t, today.AddDate(0, 0, daysPerWeek), end)
	assert.Equal(t, []string{slotBreakfast}, pastSlots)
}

func TestExportWindow_AtNoonEnd(t *testing.T) {
	now := time.Date(2026, 1, 1, slotNoonEnd, 0, 0, 0, time.UTC)
	today, end, pastSlots := ExportWindow(now)
	assert.Equal(t, now.Truncate(hoursPerDay*time.Hour), today)
	assert.Equal(t, today.AddDate(0, 0, daysPerWeek), end)
	assert.Equal(t, []string{slotBreakfast, slotNoon}, pastSlots)
}

func TestExportWindow_AtEveningEnd(t *testing.T) {
	now := time.Date(2026, 1, 1, slotEveningEnd, 0, 0, 0, time.UTC)
	today, end, pastSlots := ExportWindow(now)
	assert.Equal(t, now.Truncate(hoursPerDay*time.Hour), today)
	assert.Equal(t, today.AddDate(0, 0, daysPerWeek), end)
	assert.Equal(t, []string{slotBreakfast, slotNoon, slotEvening}, pastSlots)
}
