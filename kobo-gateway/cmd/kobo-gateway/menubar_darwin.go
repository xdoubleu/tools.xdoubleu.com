//go:build darwin

package main

// #cgo LDFLAGS: -framework UserNotifications
import "C"

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/progrium/darwinkit/dispatch"
	"github.com/progrium/darwinkit/helper/action"
	"github.com/progrium/darwinkit/macos"
	"github.com/progrium/darwinkit/macos/appkit"
	"github.com/progrium/darwinkit/macos/foundation"
	"github.com/progrium/darwinkit/objc"

	"tools.xdoubleu.com/kobo-gateway/internal/kobogateway"
)

// iconTemplate is a monochrome template image; macOS tints it.
//
//go:embed assets/menubar-template.png
var iconTemplate []byte

// menubarIconSize is the glyph's point size; without it AppKit draws the 2x
// PNG at pixel size, far too large.
const menubarIconSize = 18

// Menu-bar state that survives a status-item rebuild. Only touched on the
// main AppKit queue, so no lock is needed.
//
//nolint:gochecknoglobals // must outlive runUI's setup closure, see below.
var (
	// statusItem is package-level so objc.Retain's GC finalizer never releases
	// it (and the icon vanishes) after runUI's setup closure returns.
	statusItem   appkit.StatusItem
	statusButton appkit.StatusBarButton
	statusLine   appkit.MenuItem
	// lastKoboEvent lets a rebuild restore the current state.
	lastKoboEvent kobogateway.KoboEvent
)

// init wires notify.go's seam to UNUserNotificationCenter.
//
//nolint:gochecknoinits //only way to wire notify.go's seam before main runs
func init() {
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
	macos.RunApp(func(app appkit.Application, _ *appkit.ApplicationDelegate) {
		// Accessory: no Dock icon, no app switcher entry — just the status item.
		app.SetActivationPolicy(appkit.ApplicationActivationPolicyAccessory)

		buildStatusItem(release, homeDir, execPath)
		requestNotificationAuth(execPath)

		// The OS prompt can fail silently, so back it up with our own alert on
		// first install.
		if firstLaunch && runningInAppBundle(execPath) {
			promptEnableNotifications()
		}

		// macOS can drop a status item across sleep/wake; rebuilding it is the
		// reliable fix (SetVisible isn't).
		appkit.Workspace_SharedWorkspace().NotificationCenter().
			AddObserverForNameObjectQueueUsingBlock(
				foundation.NotificationName("NSWorkspaceDidWakeNotification"),
				nil,
				foundation.OperationQueue_MainQueue(),
				func(foundation.Notification) {
					defer guard("wake-observer")

					appkit.StatusBar_SystemStatusBar().RemoveStatusItem(statusItem)
					buildStatusItem(release, homeDir, execPath)
				},
			)

		go recoverGo("stop-terminate", func() {
			<-stop

			if restarting {
				// terminate: never returns to serve()'s restart tail, so exec here.
				if err := execUpdatedBinary(); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			}

			dispatch.MainQueue().DispatchSync(func() {
				app.Terminate(nil)
			})
		})

		go watchKobos(koboEvents, release)
	})
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

// buildStatusItem creates a fresh status item and restores lastKoboEvent.
// Must run on the main AppKit queue.
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

	shortRelease := kobogateway.ShortRelease(release)
	header := appkit.NewMenuItemWithTitleActionKeyEquivalent(
		fmt.Sprintf("Kobo Gateway %s — for tools.xdoubleu.com", shortRelease),
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

// notificationSettingsURL opens System Settings' Notifications pane (no API
// deep-links to a single app).
const notificationSettingsURL = "x-apple.systempreferences:" +
	"com.apple.preference.notifications"

// promptEnableNotifications shows a first-launch alert steering the user to
// enable notifications, since the OS prompt can fail silently. Runs its own
// modal loop, so it's safe before the main run loop starts.
func promptEnableNotifications() {
	defer guard("promptEnableNotifications")

	alert := appkit.NewAlert()
	alert.SetMessageText("Enable Notifications for Kobo Gateway?")
	alert.SetInformativeText(
		"Kobo Gateway can notify you when your Kobo connects or " +
			"disconnects, and when it updates itself. Enable notifications " +
			"in System Settings to see these.",
	)
	alert.AddButtonWithTitle("Open System Settings")
	alert.AddButtonWithTitle("Not Now")

	if alert.RunModal() == appkit.AlertFirstButtonReturn {
		appkit.Workspace_SharedWorkspace().
			OpenURL(foundation.URL_URLWithString(notificationSettingsURL))
	}
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

// watchKobos shows each connect/disconnect, redispatching to the main queue.
func watchKobos(events <-chan kobogateway.KoboEvent, release string) {
	for ev := range events {
		dispatch.MainQueue().DispatchAsync(func() {
			defer guard("applyKoboEvent")

			applyKoboEvent(ev, release, true)
		})
	}
}

// applyKoboEvent updates the status item from ev and records it; notify is
// false when re-applying after a rebuild.
func applyKoboEvent(ev kobogateway.KoboEvent, release string, notify bool) {
	lastKoboEvent = ev

	statusButton.SetToolTip(kobogateway.KoboTooltip(ev, release))
	statusLine.SetTitle(kobogateway.KoboMenuLine(ev))

	if notify {
		postNotification(kobogateway.KoboNotification(ev))
	}
}

// requestNotificationAuth asks once per process. The completion handler is
// nil to avoid marshalling a Go func as an ObjC block; the result isn't needed.
func requestNotificationAuth(execPath string) {
	if !runningInAppBundle(execPath) {
		return
	}

	notifyAuthOnce.Do(func() {
		defer guard("requestNotificationAuth")

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

// postNotification posts a best-effort UNUserNotificationCenter toast via
// objc.Call (darwinkit has no UserNotifications bindings). Only works inside
// a .app bundle; it throws otherwise.
func postNotification(title, body string) {
	if !runningInAppBundle(currentExecPath()) {
		return
	}

	defer guard("postNotification")

	objc.WithAutoreleasePool(func() {
		content := objc.Call[objc.Object](
			objc.GetClass("UNMutableNotificationContent"), objc.Sel("new"),
		)
		content.Autorelease()
		objc.Call[objc.Void](content, objc.Sel("setTitle:"), title)

		if body != "" {
			objc.Call[objc.Void](content, objc.Sel("setBody:"), body)
		}

		// Unique identifier so toasts stack instead of coalescing.
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

// notificationSeqCounter backs notificationSeq.
//
//nolint:gochecknoglobals // simple monotonic counter, only ever incremented.
var notificationSeqCounter uint64

func notificationSeq() uint64 {
	notificationSeqCounter++

	return notificationSeqCounter
}

// currentExecPath re-resolves the executable path for postNotification.
func currentExecPath() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}

	return path
}
