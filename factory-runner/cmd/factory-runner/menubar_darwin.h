// C bridge between menubar_darwin.go and the AppKit code in menubar_darwin.m.
// Strings are copied before each call returns, so Go may free them after.
#ifndef FACTORY_RUNNER_MENUBAR_DARWIN_H
#define FACTORY_RUNNER_MENUBAR_DARWIN_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

// Menu item tags passed to goMenuAction.
enum {
	FRActionToggleRunner = 1,
	FRActionCheckUpdates = 2,
	FRActionToggleLogin = 3,
	FRActionQuit = 4,
};

// Main thread only.
bool frRunApp(void);
void frBuildStatusItem(const void *icon, size_t iconLen, double iconSize);
void frSetMenu(const char *header, const char *status, const char *beat,
               const char *update, bool stopped, bool loginOn);

// Any thread.
void frTerminate(void);

// Go callbacks (//export in menubar_darwin.go), called on the main thread.
void goAppDidFinishLaunching(void);
void goSystemDidWake(void);
void goMenuWillOpen(void);
void goMenuAction(int action);

#endif
