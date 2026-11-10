//go:build darwin && cgo

// UserNotifications for notification_darwin.go. Exceptions are caught because one
// escaping into Go would abort the process.
#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>

#include "notification_darwin.h"

// stringWithUTF8String: returns nil for NULL or invalid UTF-8.
static NSString *snString(const char *s) {
	NSString *str = s != NULL ? [NSString stringWithUTF8String:s] : nil;
	return str != nil ? str : @"";
}

void snRequestAuth(void) {
	@autoreleasepool {
		@try {
			[[UNUserNotificationCenter currentNotificationCenter]
			    requestAuthorizationWithOptions:(UNAuthorizationOptionAlert |
			                                     UNAuthorizationOptionSound)
			                  completionHandler:^(BOOL granted, NSError *error){
			                  }];
		} @catch (NSException *e) {
			NSLog(@"notification authorization: %@", e);
		}
	}
}

void snPost(const char *title, const char *body) {
	@autoreleasepool {
		@try {
			UNMutableNotificationContent *content =
			    [[UNMutableNotificationContent alloc] init];
			content.title = snString(title);
			if (body != NULL && body[0] != '\0') {
				content.body = snString(body);
			}
			// A unique identifier stacks toasts instead of coalescing them.
			UNNotificationRequest *request = [UNNotificationRequest
			    requestWithIdentifier:[[NSUUID UUID] UUIDString]
			                  content:content
			                  trigger:nil];
			[[UNUserNotificationCenter currentNotificationCenter]
			    addNotificationRequest:request
			     withCompletionHandler:nil];
		} @catch (NSException *e) {
			NSLog(@"post notification: %@", e);
		}
	}
}
