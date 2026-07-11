//go:build darwin

package main

import (
	_ "embed"

	"github.com/progrium/darwinkit/dispatch"
	"github.com/progrium/darwinkit/macos"
	"github.com/progrium/darwinkit/macos/appkit"
	"github.com/progrium/darwinkit/objc"
)

// iconTemplate is a monochrome PDF/PNG rendered as a status-bar template
// image (macOS tints it automatically for light/dark menu bars).
//
//go:embed assets/menubar-template.png
var iconTemplate []byte

// runUI shows a menu-bar status item so the running gateway is visible, and
// blocks until the app quits — either via the Quit menu item or stop being
// closed (self-update requesting a restart). Must run on the main OS thread
// (see runtime.LockOSThread in main).
func runUI(release string, stop <-chan struct{}) {
	macos.RunApp(func(app appkit.Application, _ *appkit.ApplicationDelegate) {
		// Accessory: no Dock icon, no app switcher entry — just the status item.
		app.SetActivationPolicy(appkit.ApplicationActivationPolicyAccessory)

		item := appkit.StatusBar_SystemStatusBar().
			StatusItemWithLength(appkit.VariableStatusItemLength)
		objc.Retain(&item)

		if len(iconTemplate) > 0 {
			img := appkit.NewImageWithData(iconTemplate)
			img.SetTemplate(true)
			item.Button().SetImage(img)
		} else {
			item.Button().SetTitle("Kobo")
		}

		menu := appkit.NewMenu()

		title := appkit.NewMenuItemWithTitleActionKeyEquivalent(
			"kobo-gateway "+release, objc.Sel(""), "")
		title.SetEnabled(false)
		menu.AddItem(title)

		menu.AddItem(appkit.MenuItem_SeparatorItem())

		quit := appkit.NewMenuItemWithTitleActionKeyEquivalent(
			"Quit", objc.Sel("terminate:"), "q")
		menu.AddItem(quit)

		item.SetMenu(menu)

		go func() {
			<-stop
			dispatch.MainQueue().DispatchSync(func() {
				app.Terminate(nil)
			})
		}()
	})
}
