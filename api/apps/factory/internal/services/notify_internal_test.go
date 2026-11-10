package services

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFirstLineIsTheStartOfTheFirstLineOnly(t *testing.T) {
	assert.Equal(t, "Did it.", firstLine("  Did it.\nSecret details\nmore"))
	assert.Empty(t, firstLine("  \n "))
	long := strings.Repeat("é", maxNotifyDetail+5)
	assert.Equal(t, strings.Repeat("é", maxNotifyDetail)+"…", firstLine(long))
	assert.Equal(
		t,
		strings.Repeat("é", maxNotifyDetail),
		firstLine(strings.Repeat("é", maxNotifyDetail)),
	)
}

type recorder struct{ texts []string }

func (r *recorder) Notify(
	_ context.Context,
	text string,
) {
	r.texts = append(r.texts, text)
}

func TestATaskLinkEscapesItsTitleAndPointsAtTheBoard(t *testing.T) {
	n := notices{out: nil, webURL: "https://tools.example.com"}
	assert.Equal(
		t,
		"<https://tools.example.com/factory?task=T-142|Fix &lt;!here&gt;>",
		n.taskLink(142, "Fix <!here>"),
	)
}

func TestNoticesWithoutANotifierSendNothing(t *testing.T) {
	notices{out: nil, webURL: ""}.say(context.Background(), "x")

	rec := &recorder{texts: nil}
	notices{out: rec, webURL: ""}.say(context.Background(), "hello")
	assert.Equal(t, []string{"hello"}, rec.texts)
}
