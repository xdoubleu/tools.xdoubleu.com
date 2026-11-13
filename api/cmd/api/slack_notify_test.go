package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/kit/logging"
)

func TestSlackNotifyNeedsAnHTTPSWebhook(t *testing.T) {
	app := new(Application)
	app.logger = logging.NewNopLogger()

	assert.Nil(t, app.slackNotify(), "no webhook")

	app.config.FactorySlackWebhookURL = "http://hooks.slack.com/services/x"
	assert.Nil(t, app.slackNotify(), "not https")

	app.config.FactorySlackWebhookURL = "https://hooks.slack.com/services/x"
	assert.NotNil(t, app.slackNotify())
}
