//go:build !darwin || !cgo

package main

import "context"

// runMenu has no menu bar without cgo and AppKit; it just runs work.
func runMenu(ctx context.Context, _ menuInfo, work func(context.Context) error) error {
	return work(ctx)
}
