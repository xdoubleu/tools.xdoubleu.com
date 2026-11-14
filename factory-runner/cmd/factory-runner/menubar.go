package main

import (
	"context"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// menuInfo is what the menu bar needs to show and drive the runner.
type menuInfo struct {
	Control  *runner.Control
	Release  string
	Home     string
	ExecPath string
	// MaxJobs reads how many jobs run at once; SetMaxJobs saves the owner's choice.
	MaxJobs    func() int
	SetMaxJobs func(int) error
	// Mode reads the configured mode; SetMode saves the owner's choice.
	Mode    func() string
	SetMode func(string) error
	// Repos reads the allowlist; AddRepo and RemoveRepo edit it.
	Repos      func() []string
	AddRepo    func(string) error
	RemoveRepo func(string) error
	// Server reads the API root; Pair swaps a pairing code for a device token,
	// first pointing the runner at server when it is not empty.
	Server func() string
	Pair   func(ctx context.Context, code, server string) error
}

// menuFunc runs work beside the menu bar icon and returns work's error. It ends
// work (by cancelling its context) when the owner quits from the menu.
type menuFunc func(
	ctx context.Context, info menuInfo, work func(context.Context) error,
) error
