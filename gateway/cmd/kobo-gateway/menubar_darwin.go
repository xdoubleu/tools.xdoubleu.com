//go:build darwin

package main

import (
	_ "embed"
	"fmt"
	"os"

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

// statusItem holds the menu-bar status item at package scope. objc.Retain
// (below) retains the underlying NSStatusItem but also installs a Go
// finalizer that releases it once its Go wrapper is garbage-collected — a
// local variable inside runUI's setup closure becomes unreachable as soon as
// that closure returns, so the item would be finalized (and the icon vanish)
// a few GC cycles after launch. Keeping a package-level reference to it
// prevents that GC.
//
//nolint:gochecknoglobals // must outlive runUI's setup closure, see above.
var statusItem appkit.StatusItem

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

		statusItem = appkit.StatusBar_SystemStatusBar().
			StatusItemWithLength(appkit.VariableStatusItemLength)
		objc.Retain(&statusItem)

		button := statusItem.Button()
		if len(iconTemplate) > 0 {
			img := appkit.NewImageWithData(iconTemplate)
			img.SetTemplate(true)
			img.SetSize(foundation.Size{Width: menubarIconSize, Height: menubarIconSize})
			button.SetImage(img)
		} else {
			button.SetTitle("Kobo")
		}
		button.SetToolTip(kobogateway.KoboTooltip(kobogateway.KoboEvent{}, release))

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

		status := appkit.NewMenuItemWithTitleActionKeyEquivalent(
			"No Kobo connected", objc.Sel(""), "")
		status.SetEnabled(false)
		menu.AddItem(status)

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

		go func() {
			<-stop
			dispatch.MainQueue().DispatchSync(func() {
				app.Terminate(nil)
			})
		}()

		go watchKobos(koboEvents, button, status, release)
	})
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
func watchKobos(
	events <-chan kobogateway.KoboEvent,
	button appkit.StatusBarButton,
	status appkit.MenuItem,
	release string,
) {
	for ev := range events {
		ev := ev
		dispatch.MainQueue().DispatchAsync(func() {
			applyKoboEvent(ev, button, status, release)
		})
	}
}

func applyKoboEvent(
	ev kobogateway.KoboEvent,
	button appkit.StatusBarButton,
	status appkit.MenuItem,
	release string,
) {
	button.SetToolTip(kobogateway.KoboTooltip(ev, release))
	status.SetTitle(kobogateway.KoboMenuLine(ev))
	postNotification(kobogateway.KoboNotification(ev))
}

// postNotification shows a best-effort local notification via the legacy
// NSUserNotification API. darwinkit doesn't generate bindings for deprecated
// APIs (see its own notification example) or for UNUserNotificationCenter
// (which needs a proper signed app bundle/entitlement this ad-hoc-signed
// .app doesn't have), so this calls NSUserNotification directly through
// objc.Call, same as darwinkit's own example does.
//
// ponytail: NSUserNotification is deprecated but still functional on
// current macOS; move to UNUserNotificationCenter if Apple ever removes it.
func postNotification(title, body string) {
	objc.WithAutoreleasePool(func() {
		notif := objc.Call[objc.Object](objc.GetClass("NSUserNotification"), objc.Sel("new"))
		notif.Autorelease()
		objc.Call[objc.Void](notif, objc.Sel("setTitle:"), title)

		if body != "" {
			objc.Call[objc.Void](notif, objc.Sel("setInformativeText:"), body)
		}

		center := objc.Call[objc.Object](
			objc.GetClass("NSUserNotificationCenter"),
			objc.Sel("defaultUserNotificationCenter"),
		)
		objc.Call[objc.Void](center, objc.Sel("deliverNotification:"), notif)
	})
}
