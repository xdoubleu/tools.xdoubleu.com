package books

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/books/internal/services"
)

func TestReaders(t *testing.T) {
	n, m := 40, 900
	//nolint:exhaustruct // only what readers reads
	unicat := services.SourceProposal{Source: "unicat", Title: "Dune"}
	//nolint:exhaustruct // only what readers reads
	hardcover := services.SourceProposal{Source: "hardcover", Title: "dune", Readers: &m}
	//nolint:exhaustruct // only what readers reads
	other := services.SourceProposal{
		Source: "hardcover", Title: "Dune Messiah", Readers: &n,
	}

	assert.Equal(t, &m, readers([]services.SourceProposal{hardcover}, hardcover))
	assert.Equal(t, &m, readers(
		[]services.SourceProposal{unicat, other, hardcover}, unicat,
	), "borrowed from the same title")
	assert.Nil(t, readers([]services.SourceProposal{unicat, other}, unicat))
}
