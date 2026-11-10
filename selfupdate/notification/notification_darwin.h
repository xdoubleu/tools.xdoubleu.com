// C bridge between notification_darwin.go and notification_darwin.m. Strings are
// copied before each call returns, so Go may free them after.
#ifndef SELFUPDATE_NOTIFICATION_DARWIN_H
#define SELFUPDATE_NOTIFICATION_DARWIN_H

// Only inside a .app bundle; any thread.
void snRequestAuth(void);
void snPost(const char *title, const char *body);

#endif
