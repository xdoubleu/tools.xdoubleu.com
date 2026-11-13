package services

import (
	"context"
	"strings"
	"unicode/utf8"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
	"tools.xdoubleu.com/internal/platform/slack"
)

// maxNotifyDetail caps the outcome quoted in a notification.
const maxNotifyDetail = 200

// Notifier tells the owner that something needs them; Slack, in production. A
// notification never fails the work it reports.
type Notifier interface {
	Notify(ctx context.Context, text string)
}

// notices sends the factory's messages. The zero value, with no Notifier, sends
// nothing.
type notices struct {
	out    Notifier
	webURL string
}

// firstLine is the start of text's first line, cut to maxNotifyDetail runes.
func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	if utf8.RuneCountInString(line) > maxNotifyDetail {
		line = string([]rune(line)[:maxNotifyDetail]) + "…"
	}
	return line
}

// taskLink is a Slack link to the task on the board, labelled with its title.
func (n notices) taskLink(number int64, title string) string {
	return "<" + n.webURL + "/factory?task=" + models.TaskRef(number) + "|" + slack.Escape(
		title,
	) + ">"
}

// say sends text if a Notifier is set.
func (n notices) say(ctx context.Context, text string) {
	if n.out != nil {
		n.out.Notify(ctx, text)
	}
}
