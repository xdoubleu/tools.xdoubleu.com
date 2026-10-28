//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework UserNotifications
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
	"runtime/cgo"
	"sync"
	"syscall"
	"unsafe"

	"tools.xdoubleu.com/kobo-gateway/internal/kobogateway"
)

// iconTemplate is a monochrome template image; macOS tints it.
//
//go:embed assets/menubar-template.png
var iconTemplate []byte

// menubarIconSize is the glyph's point size; without it AppKit draws the 2x
// PNG at pixel size, far too large.
const menubarIconSize = 18

// uiState is runUI's arguments, for the //export callbacks.
type uiState struct {
	release, homeDir, execPath string
	firstLaunch                bool
	stop                       <-chan struct{}
	koboEvents                 <-chan kobogateway.KoboEvent
}

// Menu-bar state for the AppKit callbacks. Only touched on the main thread,
// so no lock is needed.
//
//nolint:gochecknoglobals // the //export callbacks take no arguments.
var (
	ui uiState
	// lastKoboEvent lets a status-item rebuild restore the current state.
	lastKoboEvent kobogateway.KoboEvent
)

// init wires notify.go's seam to UNUserNotificationCenter and keeps main on
// the main OS thread, which AppKit requires (locking in main is too late).
//
//nolint:gochecknoinits //only way to lock the main thread before main runs
func init() {
	runtime.LockOSThread()

	notify = postNotification
}

// notifyAuthOnce requests notification authorization once per process.
//
//nolint:gochecknoglobals // one-shot guard, see requestNotificationAuth.
var notifyAuthOnce sync.Once

// runUI shows the menu-bar status item and blocks until the app quits. Must
// run on the main OS thread.
func runUI(
	release string,
	stop <-chan struct{},
	koboEvents <-chan kobogateway.KoboEvent,
	homeDir, execPath string,
	firstLaunch bool,
) {
	ui = uiState{
		release:     release,
		homeDir:     homeDir,
		execPath:    execPath,
		firstLaunch: firstLaunch,
		stop:        stop,
		koboEvents:  koboEvents,
	}

	if !C.kgRunApp() {
		fmt.Fprintln(os.Stderr, "kobo-gateway: not on the main thread, no menu bar")
		<-stop
	}
}

//export goAppDidFinishLaunching
func goAppDidFinishLaunching() {
	buildStatusItem()
	requestNotificationAuth(ui.execPath)

	// The OS prompt can fail silently, so back it up with our own alert on
	// first install.
	if ui.firstLaunch && runningInAppBundle(ui.execPath) {
		promptEnableNotifications()
	}

	stop := ui.stop
	go recoverGo("stop-terminate", func() {
		<-stop

		if restarting {
			// terminate: never returns to serve()'s restart tail, so exec here.
			if err := execUpdatedBinary(); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}

		C.kgTerminate()
	})

	go watchKobos(ui.koboEvents)
}

//export goSystemDidWake
func goSystemDidWake() {
	defer guard("wake-observer")

	buildStatusItem()
}

//export goMenuAction
func goMenuAction(action C.int) {
	defer guard("menu-action")

	switch action {
	case C.KGActionOpenSite:
		openURL(kobogateway.DefaultWebOrigin)
	case C.KGActionToggleLoginItem:
		toggleLoginItem(ui.homeDir, ui.execPath)
	}
}

// runOnMain queues fn on the main thread, like dispatch_async to the main
// queue; goRunHandle runs it.
func runOnMain(fn func()) {
	C.kgDispatchMain(C.uintptr_t(cgo.NewHandle(fn)))
}

//export goRunHandle
func goRunHandle(handle C.uintptr_t) {
	h := cgo.Handle(handle)
	defer h.Delete()

	if fn, ok := h.Value().(func()); ok {
		fn()
	}
}

// execUpdatedBinary relaunches the updated binary. In a .app bundle it uses
// `open -n`: an in-place syscall.Exec bypasses LaunchServices, breaking
// NSStatusItem and notifications. A dev binary falls back to syscall.Exec.
// Duplicates serve()'s restart tail on purpose: this file is excluded from
// coverage, so sharing would churn main.go's patch coverage.
func execUpdatedBinary() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not resolve executable to restart: %w", err)
	}

	if appDir := kobogateway.AppBundlePath(executable); appDir != "" {
		fmt.Fprintln(os.Stdout, "updated, relaunching app bundle…")

		return exec.CommandContext(context.Background(), "open", "-n", appDir).Start()
	}

	fmt.Fprintln(os.Stdout, "updated, restarting into the new binary…")

	return syscall.Exec(executable, os.Args, os.Environ())
}

// buildStatusItem replaces the status item and restores lastKoboEvent. Must
// run on the main thread.
func buildStatusItem() {
	header := C.CString(fmt.Sprintf(
		"Kobo Gateway %s — for tools.xdoubleu.com",
		kobogateway.ShortRelease(ui.release),
	))
	defer C.free(unsafe.Pointer(header))

	var icon unsafe.Pointer
	if len(iconTemplate) > 0 {
		icon = unsafe.Pointer(&iconTemplate[0])
	}

	C.kgBuildStatusItem(
		header,
		icon,
		C.size_t(len(iconTemplate)),
		C.double(menubarIconSize),
		C.bool(kobogateway.LoginItemEnabled(ui.homeDir)),
	)

	applyKoboEvent(lastKoboEvent, false)
}

// notificationSettingsURL opens System Settings' Notifications pane (no API
// deep-links to a single app).
const notificationSettingsURL = "x-apple.systempreferences:" +
	"com.apple.preference.notifications"

// promptEnableNotifications shows a first-launch alert steering the user to
// enable notifications, since the OS prompt can fail silently.
func promptEnableNotifications() {
	defer guard("promptEnableNotifications")

	message := C.CString("Enable Notifications for Kobo Gateway?")
	defer C.free(unsafe.Pointer(message))
	info := C.CString(
		"Kobo Gateway can notify you when your Kobo connects or " +
			"disconnects, and when it updates itself. Enable notifications " +
			"in System Settings to see these.",
	)
	defer C.free(unsafe.Pointer(info))
	openSettings := C.CString("Open System Settings")
	defer C.free(unsafe.Pointer(openSettings))
	notNow := C.CString("Not Now")
	defer C.free(unsafe.Pointer(notNow))

	if C.kgRunAlert(message, info, openSettings, notNow) {
		openURL(notificationSettingsURL)
	}
}

func openURL(url string) {
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))

	C.kgOpenURL(cURL)
}

func toggleLoginItem(homeDir, execPath string) {
	var err error
	if kobogateway.LoginItemEnabled(homeDir) {
		err = kobogateway.DisableLoginItem(homeDir)
	} else {
		err = kobogateway.EnableLoginItem(homeDir, execPath)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "toggle login item:", err)
	}

	C.kgSetLoginItemState(C.bool(kobogateway.LoginItemEnabled(homeDir)))
}

// watchKobos shows each connect/disconnect, redispatching to the main thread.
func watchKobos(events <-chan kobogateway.KoboEvent) {
	for ev := range events {
		runOnMain(func() {
			defer guard("applyKoboEvent")

			applyKoboEvent(ev, true)
		})
	}
}

// applyKoboEvent updates the status item from ev and records it; notify is
// false when re-applying after a rebuild. Must run on the main thread.
func applyKoboEvent(ev kobogateway.KoboEvent, notify bool) {
	lastKoboEvent = ev

	tooltip := C.CString(kobogateway.KoboTooltip(ev, ui.release))
	defer C.free(unsafe.Pointer(tooltip))
	line := C.CString(kobogateway.KoboMenuLine(ev))
	defer C.free(unsafe.Pointer(line))

	C.kgSetStatus(tooltip, line)

	if notify {
		postNotification(kobogateway.KoboNotification(ev))
	}
}

// requestNotificationAuth asks once per process.
func requestNotificationAuth(execPath string) {
	if !runningInAppBundle(execPath) {
		return
	}

	notifyAuthOnce.Do(func() {
		C.kgRequestNotificationAuth()
	})
}

// postNotification posts a best-effort UNUserNotificationCenter toast. Safe
// from any goroutine; skipped outside a .app bundle, where it throws.
func postNotification(title, body string) {
	if !runningInAppBundle(currentExecPath()) {
		return
	}

	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	cBody := C.CString(body)
	defer C.free(unsafe.Pointer(cBody))

	C.kgPostNotification(cTitle, cBody)
}

// currentExecPath re-resolves the executable path for postNotification.
func currentExecPath() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}

	return path
}
