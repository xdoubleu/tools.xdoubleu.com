package factory

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

type noteTaker struct{}

func (noteTaker) Notify(context.Context, string) {}

func TestConnectSlackBuildsASenderOnlyForAnHTTPSAddress(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	none := new(Factory)
	none.connectSlack(logger)
	assert.Nil(t, none.notifier, "no address, no sender")
	assert.Empty(t, logs.String(), "and nothing to warn about")

	plain := new(Factory)
	plain.slackURL = "http://hooks.slack.com/x"
	plain.connectSlack(logger)
	assert.Nil(t, plain.notifier)
	assert.Contains(t, logs.String(), "not an https address")

	secure := new(Factory)
	secure.slackURL = "https://hooks.slack.com/x"
	secure.connectSlack(logger)
	assert.NotNil(t, secure.notifier)

	own := new(Factory)
	own.slackURL = "https://hooks.slack.com/x"
	own.notifier = noteTaker{}
	own.connectSlack(logger)
	assert.Equal(t, noteTaker{}, own.notifier, "a notifier set by hand is kept")
}
