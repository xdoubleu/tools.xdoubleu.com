package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/internal/kit/database"
)

// InboundEmail is the metadata of an email Resend received; the body is
// fetched separately.
type InboundEmail struct {
	EmailID   string
	MessageID string
	Subject   string
	To        []string
	// ReceivedFor is the address the route matched, which differs from To
	// when the alias was cc'd/bcc'd; also tried when resolving the feed.
	ReceivedFor []string
}

// HandleInboundEmail ingests a received email into the email feed its alias
// belongs to. Every failure is logged or recorded on the feed, never
// returned: the webhook must still be acked so Resend doesn't retry forever.
func (s *FeedService) HandleInboundEmail(ctx context.Context, in InboundEmail) {
	candidates := append(append([]string{}, in.To...), in.ReceivedFor...)
	feed := s.resolveEmailFeed(ctx, candidates)
	if feed == nil {
		s.logger.WarnContext(ctx, "email inbound: unknown alias",
			"to", in.To, "receivedFor", in.ReceivedFor)
		return
	}

	if s.mail == nil {
		s.recordEmailFetchFailure(ctx, feed.ID, errors.New("resend not configured"))
		return
	}
	htmlBody, err := s.mail.HTML(ctx, in.EmailID)
	if err != nil {
		s.logger.ErrorContext(ctx, "email inbound: fetch body failed",
			"feedID", feed.ID, "emailID", in.EmailID, "error", err)
		s.recordEmailFetchFailure(ctx, feed.ID, err)
		return
	}

	messageID := in.MessageID
	if messageID == "" {
		messageID = in.EmailID
	}
	s.ingestEmail(ctx, *feed, messageID, in.Subject, htmlBody)
}

// resolveEmailFeed returns the first known email feed among the candidate
// addresses, or nil.
func (s *FeedService) resolveEmailFeed(
	ctx context.Context,
	to []string,
) *models.Feed {
	for _, addr := range to {
		token, ok := inboundTokenFromAddress(addr)
		if !ok {
			continue
		}
		h := sha256.Sum256([]byte(token))
		feed, err := s.feeds.GetByInboundTokenHash(ctx, hex.EncodeToString(h[:]))
		if err == nil {
			return feed
		}
		if !errors.Is(err, database.ErrResourceNotFound) {
			s.logger.WarnContext(ctx, "email inbound: token lookup failed",
				"error", err)
		}
	}
	return nil
}

// inboundTokenFromAddress extracts the token from "<token>@domain" or the
// legacy "reading+<token>@domain" form, which must keep resolving. It is
// lowercased because some relays lowercase the local-part (tokens are hex).
func inboundTokenFromAddress(addr string) (string, bool) {
	local, _, ok := strings.Cut(addr, "@")
	if !ok || local == "" {
		return "", false
	}
	if idx := strings.LastIndex(local, "+"); idx != -1 {
		local = local[idx+1:]
	}
	if local == "" {
		return "", false
	}
	return strings.ToLower(local), true
}
