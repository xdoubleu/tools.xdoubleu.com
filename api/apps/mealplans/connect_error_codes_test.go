package mealplans_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func TestErrorCodes(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ID: userID}, //nolint:exhaustruct // only ID needed
	)
	unknown := uuid.NewString()

	tests := []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"get unknown plan", func() error {
			_, err := client.GetPlan(ctx, &mealplansv1.GetPlanRequest{Id: unknown})
			return err
		}, connect.CodeNotFound},
		{"delete meal of unknown plan", func() error {
			_, err := client.DeleteMeal(ctx, &mealplansv1.DeleteMealRequest{
				PlanId: unknown, MealId: unknown,
			})
			return err
		}, connect.CodeNotFound},
		{"move meal of unknown plan", func() error {
			_, err := client.MoveMeal(ctx, &mealplansv1.MoveMealRequest{
				PlanId: unknown, MealId: unknown,
				NewDate: "2026-01-01", NewSlot: "noon",
			})
			return err
		}, connect.CodeNotFound},
		{"invalid plan id", func() error {
			_, err := client.GetPlan(ctx, &mealplansv1.GetPlanRequest{Id: "nope"})
			return err
		}, connect.CodeInvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, connect.CodeOf(tt.call()))
		})
	}
}
