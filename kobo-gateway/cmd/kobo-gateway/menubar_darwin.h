// C bridge between menubar_darwin.go and the AppKit code in menubar_darwin.m.
// Strings are copied before each call returns, so Go may free them after.
#ifndef KOBO_GATEWAY_MENUBAR_DARWIN_H
#define KOBO_GATEWAY_MENUBAR_DARWIN_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

// Menu item tags passed to goMenuAction.
enum {
	KGActionOpenSite = 1,
	KGActionToggleLoginItem = 2,
};

// Main thread only.
bool kgRunApp(void);
void kgBuildStatusItem(const char *header, const void *icon, size_t iconLen,
                       double iconSize, bool loginItemOn);
void kgSetStatus(const char *tooltip, const char *line);
void kgSetLoginItemState(bool on);
bool kgRunAlert(const char *message, const char *info, const char *firstButton,
                const char *secondButton);
void kgOpenURL(const char *url);

// Any thread.
void kgDispatchMain(uintptr_t handle);
void kgTerminate(void);
void kgRequestNotificationAuth(void);
void kgPostNotification(const char *title, const char *body);

// Go callbacks (//export in menubar_darwin.go), called on the main thread.
void goAppDidFinishLaunching(void);
void goSystemDidWake(void);
void goMenuAction(int action);
void goRunHandle(uintptr_t handle);

#endif
