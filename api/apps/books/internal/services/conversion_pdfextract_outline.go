package services

import (
	"fmt"
	"html"
	"strings"
	"unicode"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
)

const (
	// maxTOCLevel is the deepest heading level outline entries map to.
	maxTOCLevel = 3
	// outlineMinMatchShare: blocks anchor an outline entry when their text
	// covers this share of its title.
	outlineMinMatchShare = 0.4
	// decorationChars are trimmed off a title line's ends ("— ONE —").
	decorationChars = "—–-•*· "
)

// outlineEntry is one PDF bookmark: its title, depth (1 = top level), and
// the page it points at.
type outlineEntry struct {
	title string
	level int
	page  int
}

// readOutline flattens the PDF's bookmarks in reading order, dropping
// entries that point at no page.
func readOutline(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT) []outlineEntry {
	resp, err := instance.GetBookmarks(&requests.GetBookmarks{Document: doc})
	if err != nil {
		return nil
	}
	var entries []outlineEntry
	var walk func(bookmarks []responses.GetBookmarksBookmark, level int)
	walk = func(bookmarks []responses.GetBookmarksBookmark, level int) {
		for _, b := range bookmarks {
			dest := b.DestInfo
			if dest == nil && b.ActionInfo != nil {
				dest = b.ActionInfo.DestInfo
			}
			if dest != nil {
				entries = append(entries, outlineEntry{
					title: b.Title, level: level, page: dest.PageIndex,
				})
			}
			walk(b.Children, level+1)
		}
	}
	walk(resp.Bookmarks, 1)
	return entries
}

// applyOutline marks the blocks each outline entry names as its heading:
// the run of text blocks on the entry's page whose text makes up its title
// ("PART ONE" + "System Structure and Behavior"), merged into one block. An
// entry with no such blocks gets a heading block of its own at the top of
// its page.
func applyOutline(pages [][]htmlBlock, outline []outlineEntry) [][]htmlBlock {
	for _, e := range outline {
		if e.page < 0 || e.page >= len(pages) {
			continue
		}
		title := strings.Join(strings.Fields(html.UnescapeString(e.title)), " ")
		level := min(e.level, maxTOCLevel)
		blocks := pages[e.page]
		if start, end, ok := findTitleBlocks(blocks, compactText(title)); ok {
			heading := blocks[start]
			var parts []string
			for _, b := range blocks[start:end] {
				parts = append(parts, strings.Trim(b.text, decorationChars))
			}
			heading.text = strings.Join(parts, " ")
			heading.tocLevel = level
			heading.listItem = false
			pages[e.page] = append(
				append(blocks[:start:start], heading),
				blocks[end:]...)
			continue
		}
		heading := outlineHeading(title, level)
		pages[e.page] = append([]htmlBlock{heading}, blocks...)
	}
	return pages
}

func outlineHeading(title string, level int) htmlBlock {
	return htmlBlock{
		html: "", tag: "p", text: title, medHeight: 0, isText: true,
		listItem: false, src: "", inline: "", tocLevel: level, page: 0,
	}
}

// findTitleBlocks finds the first run [start, end) of unclaimed text blocks
// whose compacted texts, concatenated, appear in the compacted title and
// cover enough of it.
func findTitleBlocks(blocks []htmlBlock, title string) (int, int, bool) {
	need := outlineMinMatchShare * float64(len(title))
	for start, b := range blocks {
		if !b.isText || b.tocLevel > 0 {
			continue
		}
		combined := compactText(b.text)
		if combined == "" || !strings.Contains(title, combined) {
			continue
		}
		end := start + 1
		for end < len(blocks) && blocks[end].isText && blocks[end].tocLevel == 0 {
			next := combined + compactText(blocks[end].text)
			if next == combined || !strings.Contains(title, next) {
				break
			}
			combined = next
			end++
		}
		if float64(len(combined)) >= need {
			return start, end, true
		}
	}
	return 0, 0, false
}

// compactText lowercases s and keeps only its letters and digits, so text
// compares regardless of spacing, punctuation, and line breaks.
func compactText(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// tocClass marks the headings the nav document lists.
const tocClass = "toc"

// outlineDemotedTag is where a heuristic heading goes when the PDF has an
// outline: below every outline level, so it stays a heading but leaves the
// TOC to the outline.
const outlineDemotedTag = "h4"

// applyOutlineHeadings renders outline headings at their outline level,
// marked for the TOC, and demotes the heuristic headings of an outlined PDF.
func applyOutlineHeadings(blocks []htmlBlock, outlined bool) {
	for i, b := range blocks {
		switch {
		case b.tocLevel > 0:
			tag := fmt.Sprintf("h%d", b.tocLevel)
			blocks[i].tag = tag
			blocks[i].html = fmt.Sprintf(
				`<%s class="%s">%s</%s>`, tag, tocClass, escapeXMLText(b.text), tag,
			)
		case outlined && (b.tag == "h1" || b.tag == "h2"):
			blocks[i].tag = outlineDemotedTag
			blocks[i].html = fmt.Sprintf(
				"<%s>%s</%s>",
				outlineDemotedTag,
				escapeXMLText(b.text),
				outlineDemotedTag,
			)
		}
	}
}
