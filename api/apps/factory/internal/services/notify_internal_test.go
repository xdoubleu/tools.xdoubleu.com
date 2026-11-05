package services

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
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
	id := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	n := notices{out: nil, webURL: "https://tools.example.com"}
	assert.Equal(
		t,
		"<https://tools.example.com/factory?task="+id.String()+"|Fix &lt;!here&gt;>",
		n.taskLink(id, "Fix <!here>"),
	)
}

func TestNoticesWithoutANotifierSendNothing(t *testing.T) {
	notices{out: nil, webURL: ""}.say(context.Background(), "x")

	rec := &recorder{texts: nil}
	notices{out: rec, webURL: ""}.say(context.Background(), "hello")
	assert.Equal(t, []string{"hello"}, rec.texts)
}
