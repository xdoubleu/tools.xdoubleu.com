package notification_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/selfupdate/notification"
)

// The test binary is not in a .app bundle, where UserNotifications would throw,
// so every call must be a harmless no-op.
func TestOutsideABundleNothingIsShown(t *testing.T) {
	assert.False(t, notification.Available())
	notification.RequestAuth()
	notification.Post("title", "body")
}
