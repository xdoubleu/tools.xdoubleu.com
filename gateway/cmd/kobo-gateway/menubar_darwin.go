//go:build darwin

package main

// #cgo LDFLAGS: -framework UserNotifications
import "C"

import (
	_ "embed"
	"fmt"
	"os"
	"sync"

	"github.com/progrium/darwinkit/dispatch"
	"github.com/progrium/darwinkit/helper/action"
	"github.com/progrium/darwinkit/macos"
	"github.com/progrium/darwinkit/macos/appkit"
	"github.com/progrium/darwinkit/macos/foundation"
	"github.com/progrium/darwinkit/objc"

	"tools.xdoubleu.com/gateway/internal/kobogateway"
)

// iconTemplate is a monochrome PNG rendered as a status-bar template
// image (macOS tints it automatically for light/dark menu bars).
//
//go:embed assets/menubar-template.png
var iconTemplate []byte

// menubarIconSize is the on-screen point size of the status-bar glyph. The
// embedded PNG is drawn at 2x that for Retina displays (see
// assets/menubar-template.png, 36x36px); without an explicit size AppKit
// renders the image at its native pixel size, which towers over the rest of
// the menu bar and effectively looks blank/missing at a glance.
const menubarIconSize = 18

// The following package-level vars hold menu-bar state that must survive
// across a status-item rebuild (see buildStatusItem) and are only ever
// read/written on the main AppKit queue — either from runUI's setup (which
// itself runs on the main thread, see runtime.LockOSThread in main.go) or
// from blocks dispatched via dispatch.MainQueue()/DispatchAsync. No lock is
// needed as long as that invariant holds.
//
//nolint:gochecknoglobals // must outlive runUI's setup closure, see below.
var (
	// statusItem holds the menu-bar status item. objc.Retain (below) retains
	// the underlying NSStatusItem but also installs a Go finalizer that
	// releases it once its Go wrapper is garbage-collected — a local variable
	// inside runUI's setup closure becomes unreachable as soon as that
	// closure returns, so the item would be finalized (and the icon vanish)
	// a few GC cycles after launch. Keeping a package-level reference to it
	// prevents that GC.
	statusItem appkit.StatusItem
	// statusButton/statusLine are the live parts of the item that
	// applyKoboEvent updates on every connect/disconnect.
	statusButton appkit.StatusBarButton
	statusLine   appkit.MenuItem
	// lastKoboEvent is the most recent event applied to the status item, so
	// a rebuild (e.g. after wake) can restore it instead of resetting to
	// "No Kobo connected".
	lastKoboEvent kobogateway.KoboEvent
)

// notifyAuthOnce guards requesting notification authorization: it only
// needs to happen once per process, regardless of how many times the status
// item is rebuilt.
//
//nolint:gochecknoglobals // one-shot guard, see requestNotificationAuth.
var notifyAuthOnce sync.Once

// runUI shows a menu-bar status item so the running gateway is visible, and
// blocks until the app quits — either via the Quit menu item or the process
// being asked to stop (self-update requesting a restart). Must run on the
// main OS thread (see runtime.LockOSThread in main).
func runUI(
	release string,
	stop <-chan struct{},
	koboEvents <-chan kobogateway.KoboEvent,
	homeDir, execPath string,
) {
	macos.RunApp(func(app appkit.Application, _ *appkit.ApplicationDelegate) {
		// Accessory: no Dock icon, no app switcher entry — just the status item.
		app.SetActivationPolicy(appkit.ApplicationActivationPolicyAccessory)

		buildStatusItem(release, homeDir, execPath)
		requestNotificationAuth(execPath)

		// macOS can drop a status item's on-screen presence across
		// sleep/wake even though the Go-side reference (and the retained
		// NSStatusItem) stays alive — the well-known "icon vanishes after
		// sleep" class of bug. Rebuilding the item from scratch on wake is
		// the reliable fix; a bare SetVisible toggle does not reliably
		// bring it back.
		appkit.Workspace_SharedWorkspace().NotificationCenter().
			AddObserverForNameObjectQueueUsingBlock(
				foundation.NotificationName("NSWorkspaceDidWakeNotification"),
				nil,
				foundation.OperationQueue_MainQueue(),
				func(foundation.Notification) {
					appkit.StatusBar_SystemStatusBar().RemoveStatusItem(statusItem)
					buildStatusItem(release, homeDir, execPath)
				},
			)

		go func() {
			<-stop
			dispatch.MainQueue().DispatchSync(func() {
				app.Terminate(nil)
			})
		}()

		go watchKobos(koboEvents, release)
	})
}

// buildStatusItem creates a fresh NSStatusItem (icon, tooltip, menu) and
// installs it as the package-level statusItem, restoring lastKoboEvent so a
// rebuild (see the wake observer in runUI) doesn't reset the visible state
// back to "No Kobo connected". Must run on the main AppKit thread/queue.
func buildStatusItem(release, homeDir, execPath string) {
	statusItem = appkit.StatusBar_SystemStatusBar().
		StatusItemWithLength(appkit.VariableStatusItemLength)
	objc.Retain(&statusItem)

	statusButton = statusItem.Button()
	if len(iconTemplate) > 0 {
		img := appkit.NewImageWithData(iconTemplate)
		img.SetTemplate(true)
		img.SetSize(foundation.Size{Width: menubarIconSize, Height: menubarIconSize})
		statusButton.SetImage(img)
	} else {
		statusButton.SetTitle("Kobo")
	}

	menu := appkit.NewMenu()

	header := appkit.NewMenuItemWithTitleActionKeyEquivalent(
		fmt.Sprintf("Kobo Gateway %s — for tools.xdoubleu.com", release),
		objc.Sel(""), "")
	header.SetEnabled(false)
	menu.AddItem(header)

	openSite := appkit.NewMenuItemWithTitleActionKeyEquivalent(
		"Open tools.xdoubleu.com", objc.Sel(""), "")
	action.Set(openSite, func(objc.Object) {
		appkit.Workspace_SharedWorkspace().
			OpenURL(foundation.URL_URLWithString(kobogateway.DefaultWebOrigin))
	})
	menu.AddItem(openSite)

	menu.AddItem(appkit.MenuItem_SeparatorItem())

	statusLine = appkit.NewMenuItemWithTitleActionKeyEquivalent(
		"No Kobo connected", objc.Sel(""), "")
	statusLine.SetEnabled(false)
	menu.AddItem(statusLine)

	menu.AddItem(appkit.MenuItem_SeparatorItem())

	loginItem := appkit.NewMenuItemWithTitleActionKeyEquivalent(
		"Start at Login", objc.Sel(""), "")
	refreshLoginItemState(loginItem, homeDir)
	action.Set(loginItem, func(objc.Object) {
		toggleLoginItem(loginItem, homeDir, execPath)
	})
	menu.AddItem(loginItem)

	menu.AddItem(appkit.MenuItem_SeparatorItem())

	quit := appkit.NewMenuItemWithTitleActionKeyEquivalent(
		"Quit", objc.Sel("terminate:"), "q")
	menu.AddItem(quit)

	statusItem.SetMenu(menu)

	applyKoboEvent(lastKoboEvent, release, false)
}

func refreshLoginItemState(item appkit.MenuItem, homeDir string) {
	if kobogateway.LoginItemEnabled(homeDir) {
		item.SetState(appkit.ControlStateValueOn)
	} else {
		item.SetState(appkit.ControlStateValueOff)
	}
}

func toggleLoginItem(item appkit.MenuItem, homeDir, execPath string) {
	var err error
	if kobogateway.LoginItemEnabled(homeDir) {
		err = kobogateway.DisableLoginItem(homeDir)
	} else {
		err = kobogateway.EnableLoginItem(homeDir, execPath)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "toggle login item:", err)
	}

	refreshLoginItemState(item, homeDir)
}

// watchKobos renders each connect/disconnect on the status button's tooltip
// and the menu's status line, and posts a best-effort notification. AppKit
// calls must happen on the main queue, so each event is redispatched there.
func watchKobos(events <-chan kobogateway.KoboEvent, release string) {
	for ev := range events {
		ev := ev
		dispatch.MainQueue().DispatchAsync(func() {
			applyKoboEvent(ev, release, true)
		})
	}
}

// applyKoboEvent updates the live status button/menu line from ev and
// records it in lastKoboEvent so a status-item rebuild can restore it.
// notify controls whether a toast is posted — false when re-applying the
// last known event after a rebuild, since that isn't a new connect/disconnect.
func applyKoboEvent(ev kobogateway.KoboEvent, release string, notify bool) {
	lastKoboEvent = ev

	statusButton.SetToolTip(kobogateway.KoboTooltip(ev, release))
	statusLine.SetTitle(kobogateway.KoboMenuLine(ev))

	if notify {
		postNotification(kobogateway.KoboNotification(ev))
	}
}

// requestNotificationAuth asks the user to allow notifications, once per
// process. A nil completion handler is passed deliberately — marshalling a
// Go func as an ObjC completion block is the main risk area in this file,
// and the result isn't needed: postNotification's delivery calls are
// themselves best-effort, so whether the user granted or denied is
// discovered implicitly (granted notifications show up; denied ones don't).
func requestNotificationAuth(execPath string) {
	if !runningInAppBundle(execPath) {
		return
	}

	notifyAuthOnce.Do(func() {
		objc.WithAutoreleasePool(func() {
			center := objc.Call[objc.Object](
				objc.GetClass("UNUserNotificationCenter"),
				objc.Sel("currentNotificationCenter"),
			)
			// UNAuthorizationOptionAlert (1) | UNAuthorizationOptionSound (4).
			const authOptions = uint(1 | 4)
			objc.Call[objc.Void](
				center,
				objc.Sel("requestAuthorizationWithOptions:completionHandler:"),
				authOptions,
				objc.Object{},
			)
		})
	})
}

// postNotification shows a best-effort local notification via
// UNUserNotificationCenter (the modern, non-deprecated notification API —
// the previous implementation used NSUserNotification, which is deprecated
// and no longer reliably delivers on current macOS). darwinkit doesn't
// generate bindings for UserNotifications.framework, so this calls it
// directly through objc.Call, same approach darwinkit's own notification
// example uses for the legacy API.
//
// Notifications only work inside a real .app bundle (see
// runningInAppBundle) — UNUserNotificationCenter throws when the process
// has no bundle proxy, which is the case for a raw dev binary.
func postNotification(title, body string) {
	if !runningInAppBundle(currentExecPath()) {
		return
	}

	objc.WithAutoreleasePool(func() {
		content := objc.Call[objc.Object](objc.GetClass("UNMutableNotificationContent"), objc.Sel("new"))
		content.Autorelease()
		objc.Call[objc.Void](content, objc.Sel("setTitle:"), title)

		if body != "" {
			objc.Call[objc.Void](content, objc.Sel("setBody:"), body)
		}

		// A fresh identifier per call so toasts stack instead of replacing
		// each other (a delivered notification with a reused identifier is
		// silently coalesced/updated by UNUserNotificationCenter).
		identifier := fmt.Sprintf("kobo-gateway-%d", notificationSeq())

		request := objc.Call[objc.Object](
			objc.GetClass("UNNotificationRequest"),
			objc.Sel("requestWithIdentifier:content:trigger:"),
			identifier, content, objc.Object{},
		)

		center := objc.Call[objc.Object](
			objc.GetClass("UNUserNotificationCenter"),
			objc.Sel("currentNotificationCenter"),
		)
		objc.Call[objc.Void](
			center,
			objc.Sel("addNotificationRequest:withCompletionHandler:"),
			request, objc.Object{},
		)
	})
}

// notificationSeqCounter backs notificationSeq; see postNotification.
//
//nolint:gochecknoglobals // simple monotonic counter, only ever incremented.
var notificationSeqCounter uint64

func notificationSeq() uint64 {
	notificationSeqCounter++

	return notificationSeqCounter
}

// currentExecPath re-resolves the running executable's path for
// postNotification, which has no access to runUI's execPath parameter
// (watchKobos/applyKoboEvent don't thread it through, and threading it
// through just to gate a best-effort toast isn't worth the extra
// parameters on every call in the chain).
func currentExecPath() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}

	return path
}
