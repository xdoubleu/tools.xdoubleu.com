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
	FRActionModeOff = 5,
	FRActionModeClaude = 6,
	FRActionAddRepo = 7,
	FRActionPair = 8,
	// FRActionRemoveRepo + i removes the i-th repo.
	FRActionRemoveRepo = 200,
	// FRActionMaxJobs + n picks n jobs at once.
	FRActionMaxJobs = 100,
};

// Most jobs the menu offers at once.
#define FR_MAX_JOBS_CHOICES 8

// Main thread only.
bool frRunApp(void);
void frBuildStatusItem(const void *icon, size_t iconLen, double iconSize);
void frSetMenu(const char *header, const char *status, const char *beat,
               const char *update, bool stopped, bool loginOn, int maxJobs,
               bool modeClaude, bool unpaired);
// Rebuilds the Repos submenu: Add…, then a Remove item per repo.
void frSetRepos(const char *const *repos, int n);
// Asks for a line of text in a dialog; the result is malloc'd (free it), or
// NULL when cancelled. Main thread only.
char *frPrompt(const char *title, const char *message, const char *initial);

// Any thread.
void frTerminate(void);

// Go callbacks (//export in menubar_darwin.go), called on the main thread.
void goAppDidFinishLaunching(void);
void goSystemDidWake(void);
void goMenuWillOpen(void);
void goMenuAction(int action);

#endif
