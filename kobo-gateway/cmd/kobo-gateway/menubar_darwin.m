//go:build darwin && cgo

// AppKit status item and UserNotifications for menubar_darwin.go. Built with
// -fobjc-arc; the strong statics below keep the status item and its menu
// alive across Go calls.
#import <Cocoa/Cocoa.h>
#import <UserNotifications/UserNotifications.h>

#include "menubar_darwin.h"

@interface KGAppDelegate : NSObject <NSApplicationDelegate>
@end

static KGAppDelegate *kgDelegate;
static NSStatusItem *kgStatusItem;
static NSMenuItem *kgStatusLine;
static NSMenuItem *kgLoginItem;

// stringWithUTF8String: returns nil for NULL or invalid UTF-8, which AppKit
// setters reject.
static NSString *kgString(const char *s) {
	NSString *str = s != NULL ? [NSString stringWithUTF8String:s] : nil;
	return str != nil ? str : @"";
}

@implementation KGAppDelegate

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
	// Accessory: no Dock icon or app-switcher entry, only the status item.
	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
	goAppDidFinishLaunching();

	// macOS can drop a status item across sleep/wake; rebuilding it is the
	// reliable fix.
	[[[NSWorkspace sharedWorkspace] notificationCenter]
	    addObserver:self
	       selector:@selector(systemDidWake:)
	           name:NSWorkspaceDidWakeNotification
	         object:nil];
}

- (void)systemDidWake:(NSNotification *)notification {
	goSystemDidWake();
}

- (void)menuAction:(NSMenuItem *)sender {
	goMenuAction((int)sender.tag);
}

@end

bool kgRunApp(void) {
	if (![NSThread isMainThread]) {
		return false;
	}

	@autoreleasepool {
		[NSApplication sharedApplication];
		// NSApp.delegate is weak; kgDelegate holds it.
		kgDelegate = [[KGAppDelegate alloc] init];
		NSApp.delegate = kgDelegate;
		[NSApp run];
	}
	return true;
}

static NSMenuItem *kgMenuItem(NSString *title, NSInteger action) {
	NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title
	                                              action:NULL
	                                       keyEquivalent:@""];
	if (action != 0) {
		item.target = kgDelegate;
		item.action = @selector(menuAction:);
		item.tag = action;
	} else {
		item.enabled = NO;
	}
	return item;
}

void kgBuildStatusItem(const char *header, const void *icon, size_t iconLen,
                       double iconSize, bool loginItemOn) {
	@autoreleasepool {
		NSStatusBar *bar = [NSStatusBar systemStatusBar];
		if (kgStatusItem != nil) {
			[bar removeStatusItem:kgStatusItem];
		}
		kgStatusItem = [bar statusItemWithLength:NSVariableStatusItemLength];

		NSImage *image = nil;
		if (icon != NULL && iconLen > 0) {
			image = [[NSImage alloc]
			    initWithData:[NSData dataWithBytes:icon length:iconLen]];
		}
		if (image != nil) {
			// Template: macOS tints it. The size stops a 2x PNG drawing at
			// pixel size.
			image.template = YES;
			image.size = NSMakeSize(iconSize, iconSize);
			kgStatusItem.button.image = image;
		} else {
			kgStatusItem.button.title = @"Kobo";
		}

		NSMenu *menu = [[NSMenu alloc] init];
		// Enabled state is set explicitly per item.
		menu.autoenablesItems = NO;

		[menu addItem:kgMenuItem(kgString(header), 0)];
		[menu addItem:kgMenuItem(@"Open tools.xdoubleu.com", KGActionOpenSite)];
		[menu addItem:[NSMenuItem separatorItem]];

		kgStatusLine = kgMenuItem(@"No Kobo connected", 0);
		[menu addItem:kgStatusLine];
		[menu addItem:[NSMenuItem separatorItem]];

		kgLoginItem = kgMenuItem(@"Start at Login", KGActionToggleLoginItem);
		kgLoginItem.state =
		    loginItemOn ? NSControlStateValueOn : NSControlStateValueOff;
		[menu addItem:kgLoginItem];
		[menu addItem:[NSMenuItem separatorItem]];

		NSMenuItem *quit = [[NSMenuItem alloc] initWithTitle:@"Quit"
		                                              action:@selector(terminate:)
		                                       keyEquivalent:@"q"];
		quit.target = NSApp;
		[menu addItem:quit];

		kgStatusItem.menu = menu;
	}
}

void kgSetStatus(const char *tooltip, const char *line) {
	@autoreleasepool {
		kgStatusItem.button.toolTip = kgString(tooltip);
		kgStatusLine.title = kgString(line);
	}
}

void kgSetLoginItemState(bool on) {
	kgLoginItem.state = on ? NSControlStateValueOn : NSControlStateValueOff;
}

bool kgRunAlert(const char *message, const char *info, const char *firstButton,
                const char *secondButton) {
	@autoreleasepool {
		NSAlert *alert = [[NSAlert alloc] init];
		alert.messageText = kgString(message);
		alert.informativeText = kgString(info);
		[alert addButtonWithTitle:kgString(firstButton)];
		[alert addButtonWithTitle:kgString(secondButton)];
		return [alert runModal] == NSAlertFirstButtonReturn;
	}
}

void kgOpenURL(const char *url) {
	@autoreleasepool {
		NSURL *nsURL = [NSURL URLWithString:kgString(url)];
		if (nsURL != nil) {
			[[NSWorkspace sharedWorkspace] openURL:nsURL];
		}
	}
}

void kgDispatchMain(uintptr_t handle) {
	dispatch_async(dispatch_get_main_queue(), ^{
		goRunHandle(handle);
	});
}

void kgTerminate(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp terminate:nil];
	});
}

// UNUserNotificationCenter throws outside a .app bundle; an exception
// escaping into Go would abort the process.
void kgRequestNotificationAuth(void) {
	@autoreleasepool {
		@try {
			[[UNUserNotificationCenter currentNotificationCenter]
			    requestAuthorizationWithOptions:(UNAuthorizationOptionAlert |
			                                     UNAuthorizationOptionSound)
			                  completionHandler:^(BOOL granted, NSError *error){
			                  }];
		} @catch (NSException *e) {
			NSLog(@"kobo-gateway: notification authorization: %@", e);
		}
	}
}

void kgPostNotification(const char *title, const char *body) {
	@autoreleasepool {
		@try {
			UNMutableNotificationContent *content =
			    [[UNMutableNotificationContent alloc] init];
			content.title = kgString(title);
			if (body != NULL && body[0] != '\0') {
				content.body = kgString(body);
			}
			// A unique identifier stacks toasts instead of coalescing them.
			NSString *identifier = [@"kobo-gateway-"
			    stringByAppendingString:[[NSUUID UUID] UUIDString]];
			UNNotificationRequest *request =
			    [UNNotificationRequest requestWithIdentifier:identifier
			                                         content:content
			                                         trigger:nil];
			[[UNUserNotificationCenter currentNotificationCenter]
			    addNotificationRequest:request
			     withCompletionHandler:nil];
		} @catch (NSException *e) {
			NSLog(@"kobo-gateway: post notification: %@", e);
		}
	}
}
