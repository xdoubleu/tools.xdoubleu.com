//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "menubar_darwin.h"
*/
import "C"

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"
	"unsafe"

	"tools.xdoubleu.com/factory-runner/internal/runner"
	"tools.xdoubleu.com/selfupdate/notification"
)

// iconTemplate is a monochrome template image; macOS tints it.
//
//go:embed assets/menubar-template.png
var iconTemplate []byte

// menubarIconSize is the glyph's point size; without it AppKit draws the 2x
// PNG at pixel size, far too large.
const menubarIconSize = 18

// ui is runMenu's argument, for the //export callbacks. Only touched on the
// main thread after runMenu sets it.
//
//nolint:gochecknoglobals // the //export callbacks take no arguments.
var ui menuInfo

// init keeps main on the main OS thread, which AppKit requires (locking in main
// is too late).
//
//nolint:gochecknoinits //only way to lock the main thread before main runs
func init() {
	runtime.LockOSThread()
}

// runMenu shows the status item and runs work beside it. Must run on the main OS
// thread. Work ending, for any reason, quits the app.
func runMenu(
	ctx context.Context,
	info menuInfo,
	work func(context.Context) error,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ui = info
	info.Control.SetNotifier(notification.Post)

	done := make(chan error, 1)
	go func() {
		done <- work(ctx)
		C.frTerminate()
	}()

	if !C.frRunApp() {
		fmt.Fprintln(os.Stderr, "factory-runner: not on the main thread, no menu bar")
		cancel()
	}
	return <-done
}

//export goAppDidFinishLaunching
func goAppDidFinishLaunching() {
	buildStatusItem()
	notification.RequestAuth()
	refreshMenu()
}

//export goSystemDidWake
func goSystemDidWake() {
	buildStatusItem()
	refreshMenu()
}

//export goMenuWillOpen
func goMenuWillOpen() {
	refreshMenu()
}

//export goMenuAction
func goMenuAction(action C.int) {
	switch action {
	case C.FRActionToggleRunner:
		if err := ui.Control.SetPaused(!ui.Control.Paused()); err != nil {
			fmt.Fprintln(os.Stderr, "factory-runner: stop/start:", err)
		}
		refreshMenu()
	case C.FRActionCheckUpdates:
		ui.Control.RequestCheck()
	case C.FRActionToggleLogin:
		toggleLoginItem()
		refreshMenu()
	case C.FRActionQuit:
		quit()
	}
}

// quit unloads the LaunchAgent, which stops this process and keeps launchd from
// starting it again until the next login. Run outside the agent (a terminal),
// there is nothing to unload, so it just terminates.
func quit() {
	target := fmt.Sprintf("gui/%d/%s", os.Getuid(), runner.LaunchAgentLabel)
	err := exec.CommandContext(context.Background(), "launchctl", "bootout", target).Run()
	if err != nil {
		C.frTerminate()
	}
}

func toggleLoginItem() {
	var err error
	if runner.LoginItemEnabled(ui.Home) {
		err = runner.DisableLoginItem(ui.Home)
	} else {
		_, err = runner.InstallLaunchAgent(ui.Home, ui.ExecPath)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "factory-runner: start at login:", err)
	}
}

func buildStatusItem() {
	var icon unsafe.Pointer
	if len(iconTemplate) > 0 {
		icon = unsafe.Pointer(&iconTemplate[0])
	}
	C.frBuildStatusItem(icon, C.size_t(len(iconTemplate)), C.double(menubarIconSize))
}

// refreshMenu writes the current status into the menu. Main thread only.
func refreshMenu() {
	s := ui.Control.Snapshot()
	lines := s.MenuLines(ui.Release, time.Now())

	header := C.CString(lines.Header)
	defer C.free(unsafe.Pointer(header))
	status := C.CString(lines.Status)
	defer C.free(unsafe.Pointer(status))
	beat := C.CString(lines.Heartbeat)
	defer C.free(unsafe.Pointer(beat))
	update := C.CString(lines.Update)
	defer C.free(unsafe.Pointer(update))

	C.frSetMenu(
		header, status, beat, update,
		C.bool(s.Paused), C.bool(runner.LoginItemEnabled(ui.Home)),
	)
}
