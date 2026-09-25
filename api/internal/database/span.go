package database

import (
	"context"

	"github.com/getsentry/sentry-go"
)

// StartSpan is used to start a [sentry.Span].
func StartSpan(ctx context.Context, dbName string, sql string) *sentry.Span {
	transaction := sentry.TransactionFromContext(ctx)

	options := []sentry.SpanOption{
		sentry.WithDescription(sql),
	}

	if transaction != nil {
		options = append(options, sentry.WithTransactionName(transaction.Name))
	}

	span := sentry.StartSpan(ctx, "db.query", options...)
	span.SetData("db.system", dbName)

	return span
}

// WrapWithSpan wraps a database action in a [sentry.Span].
func WrapWithSpan[T any](
	ctx context.Context,
	dbName string,
	queryFunc func(ctx context.Context, sql string, args ...any) (T, error),
	sql string, args ...any) (T, error) {
	span := StartSpan(ctx, dbName, sql)
	defer span.Finish()

	return queryFunc(ctx, sql, args...)
}

// WrapWithSpanNoError wraps an infallible database action in a [sentry.Span].
func WrapWithSpanNoError[T any](
	ctx context.Context,
	dbName string,
	queryFunc func(ctx context.Context, sql string, args ...any) T,
	sql string, args ...any) T {
	span := StartSpan(ctx, dbName, sql)
	defer span.Finish()

	return queryFunc(ctx, sql, args...)
}
