package unicat

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/time/rate"

	"tools.xdoubleu.com/apps/books/pkg/authorname"
)

//nolint:gochecknoglobals // overridable in tests
var baseURL = "https://www.unicat.be/sru"

//nolint:gochecknoglobals // overridable in tests
var backoffBase = 500 * time.Millisecond

//nolint:gochecknoglobals // overridable in tests
var backoffCap = 30 * time.Second

const apiTimeout = 15 * time.Second

const (
	searchLimit = 5

	isbn13Length = 13

	// UniCat publishes no rate limit; stay conservative.
	requestsPerSecond = 2
	burst             = 4

	maxAttempts = 4

	sruVersion = "1.1"
)

type client struct {
	logger     *slog.Logger
	httpClient *http.Client
	limiter    *rate.Limiter
}

// New creates a UniCat SRU client. No API key is required.
func New(logger *slog.Logger) Client {
	return client{
		logger: logger,
		httpClient: &http.Client{
			Timeout: apiTimeout,
		},
		limiter: rate.NewLimiter(requestsPerSecond, burst),
	}
}

// GetByISBN returns catalog metadata for the given ISBN-13.
func (c client) GetByISBN(ctx context.Context, isbn string) (*ExternalBook, error) {
	params := c.baseParams()
	params.Set("query", "isbn="+isbn)
	params.Set("maximumRecords", "1")

	resp, err := c.fetch(ctx, params)
	if err != nil {
		return nil, err
	}
	if resp.NumberOfRecords == 0 || len(resp.Records) == 0 {
		return nil, ErrNotFound
	}

	book := marcToExternalBook(resp.Records[0].RecordData.MarcRecord)
	return &book, nil
}

// Search queries UniCat with an "intitle:… inauthor:…" query.
func (c client) Search(ctx context.Context, query string) ([]ExternalBook, error) {
	cql := buildCQL(query)
	if cql == "" {
		return nil, nil
	}

	params := c.baseParams()
	params.Set("query", cql)
	params.Set("maximumRecords", fmt.Sprintf("%d", searchLimit))

	resp, err := c.fetch(ctx, params)
	if err != nil {
		return nil, err
	}

	books := make([]ExternalBook, 0, len(resp.Records))
	for _, r := range resp.Records {
		books = append(books, marcToExternalBook(r.RecordData.MarcRecord))
	}
	return books, nil
}

func (c client) baseParams() url.Values {
	p := url.Values{}
	p.Set("version", sruVersion)
	p.Set("operation", "searchRetrieve")
	return p
}

func (c client) fetch(ctx context.Context, params url.Values) (*sruResponse, error) {
	endpoint := baseURL + "?" + params.Encode()

	var result sruResponse
	if err := c.get(ctx, endpoint, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c client) get(ctx context.Context, endpoint string, dst *sruResponse) error {
	return c.doWithRetry(ctx, func() (bool, error) {
		if waitErr := c.limiter.Wait(ctx); waitErr != nil {
			return false, waitErr
		}

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if reqErr != nil {
			return false, reqErr
		}
		req.Header.Set("Accept", "application/xml")

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return isTransientErr(doErr), doErr
		}
		defer resp.Body.Close()

		if isRetryableStatus(resp.StatusCode) {
			raw, _ := io.ReadAll(resp.Body)
			return true, fmt.Errorf(
				"unicat SRU returned %d: %s",
				resp.StatusCode,
				string(raw),
			)
		}

		if resp.StatusCode < http.StatusOK ||
			resp.StatusCode >= http.StatusMultipleChoices {
			raw, _ := io.ReadAll(resp.Body)
			return false, fmt.Errorf(
				"unicat SRU returned %d: %s",
				resp.StatusCode,
				string(raw),
			)
		}

		return false, xml.NewDecoder(resp.Body).Decode(dst)
	})
}

func (c client) doWithRetry(
	ctx context.Context,
	attempt func() (retryable bool, err error),
) error {
	var lastErr error
	for i := range maxAttempts {
		retryable, err := attempt()
		if err == nil {
			return nil
		}

		if errors.Is(err, context.Canceled) {
			return err
		}

		lastErr = err

		if !retryable || i == maxAttempts-1 {
			break
		}

		delay := backoffDelay(i)
		c.logger.DebugContext(ctx, "retrying unicat request",
			slog.Int("attempt", i+1),
			slog.Duration("backoff", delay),
			slog.Any("error", err),
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

// marcToExternalBook maps MARC21: 245$a title (subtitle kept), 100$a/700$a
// authors, 020$a ISBN-13, 520$a description, 300$a page count.
//
//nolint:gocognit // MARC field mapping is inherently branchy
func marcToExternalBook(rec marcRecord) ExternalBook {
	var book ExternalBook

	for _, df := range rec.DataFields {
		switch df.Tag {
		case "245":
			if t := df.subfieldA(); t != "" {
				// MARC appends " /" or " :".
				cleaned := strings.TrimRight(strings.TrimSpace(t), " :/")
				book.Title = cleaned
			}
		case "100", "700":
			if a := df.subfieldA(); a != "" {
				// MARC appends "," or "." to names.
				cleaned := strings.TrimRight(strings.TrimSpace(a), ",.")
				book.Authors = append(book.Authors, authorname.Normalize(cleaned))
			}
		case "020":
			if book.ISBN13 == nil {
				if v := df.subfieldA(); v != "" {
					normalized := normalizeISBN(v)
					if len(normalized) == isbn13Length {
						book.ISBN13 = &normalized
					}
				}
			}
		case "520":
			if book.Description == nil {
				if v := df.subfieldA(); v != "" {
					book.Description = &v
				}
			}
		case "300":
			if book.PageCount == nil {
				if v := df.subfieldA(); v != "" {
					if n := parseLeadingInt(v); n > 0 {
						book.PageCount = &n
					}
				}
			}
		}
	}

	return book
}

func normalizeISBN(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func parseLeadingInt(s string) int {
	start := -1
	for i, r := range s {
		if unicode.IsDigit(r) {
			if start < 0 {
				start = i
			}
		} else if start >= 0 {
			n, err := strconv.Atoi(s[start:i])
			if err == nil {
				return n
			}
			return 0
		}
	}
	if start >= 0 {
		n, err := strconv.Atoi(s[start:])
		if err == nil {
			return n
		}
	}
	return 0
}

// buildCQL converts an intitle/inauthor query into CQL; "" means skip.
func buildCQL(query string) string {
	title := extractQuoted(query, "intitle")
	author := extractQuoted(query, "inauthor")

	if title == "" {
		return ""
	}
	if author != "" {
		return fmt.Sprintf(`title="%s" AND author="%s"`, title, author)
	}
	return fmt.Sprintf(`title="%s"`, title)
}

// extractQuoted returns the quoted value after "key:", or "".
func extractQuoted(s, key string) string {
	prefix := key + `:"`
	idx := strings.Index(s, prefix)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(prefix):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// SetBaseURL overrides the UniCat SRU base URL. Tests only.
func SetBaseURL(u string) { baseURL = u }

// SetBackoffBase overrides the backoff base delay. Tests only.
func SetBackoffBase(d time.Duration) { backoffBase = d }

func backoffDelay(attempt int) time.Duration {
	d := backoffBase * (1 << attempt)
	if d > backoffCap {
		return backoffCap
	}
	return d
}

func isRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests ||
		(status >= http.StatusInternalServerError && status < 600)
}

func isTransientErr(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Timeout()
	}
	return false
}
