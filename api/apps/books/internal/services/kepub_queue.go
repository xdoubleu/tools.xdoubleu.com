package services

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/repositories"
)

// kepubDrainBatch bounds one drain run so a long PDF conversion backlog
// doesn't hold the job slot indefinitely.
const kepubDrainBatch = 3

// KEPUBQueue hands KEPUB conversions from request handlers (web role) to the
// workers role, which drains them through ConversionService.
type KEPUBQueue struct {
	logger     *slog.Logger
	requests   *repositories.KEPUBRequestsRepository
	conversion *ConversionService
}

func NewKEPUBQueue(
	logger *slog.Logger,
	requests *repositories.KEPUBRequestsRepository,
	conversion *ConversionService,
) *KEPUBQueue {
	return &KEPUBQueue{logger: logger, requests: requests, conversion: conversion}
}

// Request records a pending conversion for the workers role; repeated
// requests for the same book collapse into one.
func (q *KEPUBQueue) Request(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	return q.requests.Enqueue(ctx, userID, bookID)
}

// Drain converts up to kepubDrainBatch pending books and returns how many it
// attempted. A request is removed once attempted: a failed conversion is
// recorded on its KEPUB row and retried only when requested again.
func (q *KEPUBQueue) Drain(ctx context.Context) (int, error) {
	pending, err := q.requests.ListOldest(ctx, kepubDrainBatch)
	if err != nil {
		return 0, err
	}
	for _, req := range pending {
		if _, convErr := q.conversion.EnsureKEPUB(
			ctx, req.UserID, req.BookID,
		); convErr != nil {
			q.logger.WarnContext(ctx, "queued kepub conversion failed",
				"book_id", req.BookID, "err", convErr)
		}
		if err = q.requests.Delete(ctx, req.UserID, req.BookID); err != nil {
			return 0, err
		}
	}
	return len(pending), nil
}
