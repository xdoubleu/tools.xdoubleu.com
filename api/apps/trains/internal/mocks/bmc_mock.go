// Package mocks holds test doubles for the trains app.
package mocks

import (
	"context"

	"tools.xdoubleu.com/apps/trains/pkg/bmc"
)

// MockBMCClient is a configurable in-memory bmc.Client.
type MockBMCClient struct {
	// Result is returned from FetchStatic when Err is nil.
	Result *bmc.StaticResult
	// Err, when set, is returned instead.
	Err error
	// Calls records the options passed to each FetchStatic call.
	Calls []bmc.StaticOptions
}

// NewMockBMCClient returns a mock that serves the given zip body once, then
// reports NotModified on every subsequent call (mirroring a conditional GET
// against an unchanged daily feed).
func NewMockBMCClient(zipBody []byte) *MockBMCClient {
	return &MockBMCClient{
		//nolint:exhaustruct //validators optional
		Result: &bmc.StaticResult{Body: zipBody, ETag: `"v1"`},
		Err:    nil,
		Calls:  nil,
	}
}

func (m *MockBMCClient) FetchStatic(
	_ context.Context,
	opts bmc.StaticOptions,
) (*bmc.StaticResult, error) {
	m.Calls = append(m.Calls, opts)
	if m.Err != nil {
		return nil, m.Err
	}
	// After the first successful fetch, behave like a 304 unless the caller
	// reset Result.
	res := m.Result
	if len(m.Calls) > 1 && res != nil && !res.NotModified {
		//nolint:exhaustruct //304 carries nothing else
		return &bmc.StaticResult{NotModified: true}, nil
	}
	return res, nil
}
