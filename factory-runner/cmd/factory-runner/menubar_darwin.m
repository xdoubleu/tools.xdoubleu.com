//go:build darwin && cgo

// AppKit status item for menubar_darwin.go. Built with -fobjc-arc; the strong
// statics below keep the status item and its menu alive across Go calls.
#import <Cocoa/Cocoa.h>

#include "menubar_darwin.h"

@interface FRAppDelegate : NSObject <NSApplicationDelegate, NSMenuDelegate>
@end

static FRAppDelegate *frDelegate;
static NSStatusItem *frStatusItem;
static NSMenuItem *frHeader;
static NSMenuItem *frStatus;
static NSMenuItem *frBeat;
static NSMenuItem *frUpdate;
static NSMenuItem *frToggle;
static NSMenuItem *frLogin;

// stringWithUTF8String: returns nil for NULL or invalid UTF-8, which AppKit
// setters reject.
static NSString *frString(const char *s) {
	NSString *str = s != NULL ? [NSString stringWithUTF8String:s] : nil;
	return str != nil ? str : @"";
}

@implementation FRAppDelegate

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

// The status lines are refreshed each time the menu opens.
- (void)menuWillOpen:(NSMenu *)menu {
	goMenuWillOpen();
}

@end

bool frRunApp(void) {
	if (![NSThread isMainThread]) {
		return false;
	}

	@autoreleasepool {
		[NSApplication sharedApplication];
		// NSApp.delegate is weak; frDelegate holds it.
		frDelegate = [[FRAppDelegate alloc] init];
		NSApp.delegate = frDelegate;
		[NSApp run];
	}
	return true;
}

static NSMenuItem *frMenuItem(NSString *title, NSInteger action) {
	NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:title
	                                              action:NULL
	                                       keyEquivalent:@""];
	if (action != 0) {
		item.target = frDelegate;
		item.action = @selector(menuAction:);
		item.tag = action;
	} else {
		item.enabled = NO;
	}
	return item;
}

void frBuildStatusItem(const void *icon, size_t iconLen, double iconSize) {
	@autoreleasepool {
		NSStatusBar *bar = [NSStatusBar systemStatusBar];
		if (frStatusItem != nil) {
			[bar removeStatusItem:frStatusItem];
		}
		frStatusItem = [bar statusItemWithLength:NSVariableStatusItemLength];

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
			frStatusItem.button.image = image;
		} else {
			frStatusItem.button.title = @"Runner";
		}

		NSMenu *menu = [[NSMenu alloc] init];
		// Enabled state is set explicitly per item.
		menu.autoenablesItems = NO;
		menu.delegate = frDelegate;

		frHeader = frMenuItem(@"Factory Runner", 0);
		frStatus = frMenuItem(@"", 0);
		frBeat = frMenuItem(@"", 0);
		frUpdate = frMenuItem(@"", 0);
		[menu addItem:frHeader];
		[menu addItem:frStatus];
		[menu addItem:frBeat];
		[menu addItem:frUpdate];
		[menu addItem:[NSMenuItem separatorItem]];

		frToggle = frMenuItem(@"Stop runner", FRActionToggleRunner);
		[menu addItem:frToggle];
		[menu addItem:frMenuItem(@"Check for updates", FRActionCheckUpdates)];
		[menu addItem:[NSMenuItem separatorItem]];

		frLogin = frMenuItem(@"Start at Login", FRActionToggleLogin);
		[menu addItem:frLogin];
		[menu addItem:[NSMenuItem separatorItem]];

		// Quit unloads the LaunchAgent (see goMenuAction); a plain terminate:
		// would have launchd start the runner again.
		NSMenuItem *quit = frMenuItem(@"Quit", FRActionQuit);
		quit.keyEquivalent = @"q";
		[menu addItem:quit];

		frStatusItem.menu = menu;
	}
}

void frSetMenu(const char *header, const char *status, const char *beat,
               const char *update, bool stopped, bool loginOn) {
	@autoreleasepool {
		frHeader.title = frString(header);
		frStatus.title = frString(status);
		frBeat.title = frString(beat);
		frUpdate.title = frString(update);
		frToggle.title = stopped ? @"Start runner" : @"Stop runner";
		frLogin.state = loginOn ? NSControlStateValueOn : NSControlStateValueOff;
	}
}

// frTerminate ends [NSApp run] without exiting the process, so runMenu can
// return the work's error.
void frTerminate(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp stop:nil];
		// stop: only takes effect after the next event.
		NSEvent *wake = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
		                                   location:NSZeroPoint
		                              modifierFlags:0
		                                  timestamp:0
		                               windowNumber:0
		                                    context:nil
		                                    subtype:0
		                                      data1:0
		                                      data2:0];
		[NSApp postEvent:wake atStart:YES];
	});
}
