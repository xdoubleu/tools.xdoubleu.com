//nolint:testpackage // testing unexported service helpers
package services

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func flowBlock(text string) htmlBlock {
	return htmlBlock{
		html: "", tag: "p", text: text, medHeight: 10, isText: true, listItem: false,
		src: "",
	}
}

func flowTexts(pages [][]htmlBlock) [][]string {
	out := make([][]string, len(pages))
	for i, blocks := range pages {
		out[i] = []string{}
		for _, b := range blocks {
			out[i] = append(out[i], b.text)
		}
	}
	return out
}

func TestRemoveRunningHeaders(t *testing.T) {
	t.Parallel()

	var pages [][]htmlBlock
	for p := 12; p <= 17; p++ {
		header := fmt.Sprintf("%d PART ONE: SYSTEM STRUCTURE", p)
		if p%2 == 1 {
			header = fmt.Sprintf("CHAPTER ONE: THE BASICS %d", p)
		}
		pages = append(pages, []htmlBlock{
			flowBlock(header),
			flowBlock(fmt.Sprintf("Body paragraph on page %d.", p)),
			flowBlock(fmt.Sprintf("%d", p)),
		})
	}
	// A chapter opener has no header; its first block is the title.
	pages = append(pages, []htmlBlock{
		flowBlock("CHAPTER TWO"), flowBlock("Opening paragraph."),
	})

	got := flowTexts(removeRunningHeaders(pages, 10))
	for i := range 6 {
		assert.Equal(t, []string{fmt.Sprintf("Body paragraph on page %d.", 12+i)}, got[i])
	}
	assert.Equal(t, []string{"CHAPTER TWO", "Opening paragraph."}, got[6])
}

func TestJoinPageContinuations(t *testing.T) {
	t.Parallel()

	aside := htmlBlock{
		html: "<blockquote/>", tag: asideTag, text: "note", medHeight: 0,
		isText: false, listItem: false, src: "",
	}
	heading := flowBlock("Runaway Loops")
	heading.medHeight = 16

	pages := [][]htmlBlock{
		{flowBlock("A tree is a system, and a forest is a larger system. The earth"), aside},
		{flowBlock("is a system. So is the solar system."), flowBlock("Next paragraph.")},
		{flowBlock("It would seem as if this were circular reason-")},
		{flowBlock("ing; profits fell because investment fell.")},
		{flowBlock("A paragraph that ends cleanly.")},
		{flowBlock("new sentence fragment stays apart")},
		{heading},
		{flowBlock("because a heading is never continued.")},
	}

	got := flowTexts(joinPageContinuations(pages, 10))
	assert.Equal(t, [][]string{
		{"A tree is a system, and a forest is a larger system. The earth is a system. So is the solar system.", "note"},
		{"Next paragraph."},
		{"It would seem as if this were circular reasoning; profits fell because investment fell."},
		{},
		{"A paragraph that ends cleanly."},
		{"new sentence fragment stays apart"},
		{"Runaway Loops"},
		{"because a heading is never continued."},
	}, got)
}

// TestRemoveRunningHeaders_ShortSection: a two-page section's header recurs
// only once in folioed form, but matches the section title on the page
// before; a numbered chapter heading at title size is never a header.
func TestRemoveRunningHeaders_ShortSection(t *testing.T) {
	t.Parallel()

	title := flowBlock("A NOTE FROM THE AUTHOR")
	title.medHeight = 16
	chapter := flowBlock("1 Introduction")
	chapter.medHeight = 16
	pages := [][]htmlBlock{
		{title, flowBlock("First page of the note.")},
		{flowBlock("X A NOTE FROM THE AUTHOR"), flowBlock("Second page of the note.")},
		{chapter, flowBlock("Chapter body.")},
		{flowBlock("2 Introduction"), flowBlock("More chapter body.")},
	}

	got := flowTexts(removeRunningHeaders(pages, 10))
	assert.Equal(t, [][]string{
		{"A NOTE FROM THE AUTHOR", "First page of the note."},
		{"Second page of the note."},
		{"1 Introduction", "Chapter body."},
		{"More chapter body."},
	}, got)
}

// TestJoinPageContinuations_SkipsFootnote: a footnote at the bottom of the
// page sits between a paragraph's halves; it stays after the joined text.
func TestJoinPageContinuations_SkipsFootnote(t *testing.T) {
	t.Parallel()

	note := flowBlock("* Definitions of words in bold face can be found in the Glossary.")
	note.medHeight = 7
	pages := [][]htmlBlock{
		{flowBlock("A tree is a system. The earth"), note},
		{flowBlock("is a system.")},
	}
	got := flowTexts(joinPageContinuations(pages, 10))
	assert.Equal(t, [][]string{
		{"A tree is a system. The earth is a system.", note.text},
		{},
	}, got)
}
