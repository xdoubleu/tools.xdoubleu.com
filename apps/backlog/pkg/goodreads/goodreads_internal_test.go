package goodreads

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func makeNode(class string) *html.Node {
	return &html.Node{ //nolint:exhaustruct //only relevant fields needed
		Type: html.ElementNode,
		Data: "li",
		Attr: []html.Attribute{{Namespace: "", Key: "class", Val: class}},
	}
}

func TestIsDivider_True(t *testing.T) {
	node := makeNode("horizontalGreyDivider")
	assert.True(t, isDivider(node))
}

func TestIsDivider_False(t *testing.T) {
	node := makeNode("someOtherClass")
	assert.False(t, isDivider(node))
}

func TestIsDivider_NoAttributes(t *testing.T) {
	node := &html.Node{ //nolint:exhaustruct //only relevant fields needed
		Type:     html.ElementNode,
		DataAtom: atom.Li,
		Data:     "li",
	}
	assert.False(t, isDivider(node))
}

func makeShelfNode(shelf string) *html.Node {
	// <li><a href="?shelf=want-to-read">want to read</a></li>
	anchor := &html.Node{ //nolint:exhaustruct //only relevant fields needed
		Type: html.ElementNode,
		Data: "a",
		Attr: []html.Attribute{{Namespace: "", Key: "href", Val: "?shelf=" + shelf}},
	}
	li := &html.Node{ //nolint:exhaustruct //only relevant fields needed
		Type:       html.ElementNode,
		Data:       "li",
		FirstChild: anchor,
	}
	anchor.Parent = li
	return li
}

func TestGetShelfOrTagName(t *testing.T) {
	node := makeShelfNode("want-to-read")
	name := getShelfOrTagName(node)
	assert.NotNil(t, name)
	assert.Equal(t, "want-to-read", *name)
}
