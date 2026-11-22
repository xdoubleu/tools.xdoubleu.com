package services

import (
	"context"

	"tools.xdoubleu.com/apps/recipes/pkg/videofetch"
)

// Video is the title and description fetched from a video URL.
type Video = videofetch.Video

// Errors a video fetch can return, for adapters to map.
var (
	ErrVideoInstagram      = videofetch.ErrInstagram
	ErrVideoUnsupportedURL = videofetch.ErrUnsupportedURL
	ErrVideoNotFound       = videofetch.ErrNotFound
	ErrVideoFetch          = videofetch.ErrFetch
)

// VideoFetcher is the video port: it fetches the text of a video, so the recipe
// in it can be imported.
type VideoFetcher interface {
	Fetch(ctx context.Context, rawURL string) (*Video, error)
}

// NewVideoService returns a VideoService over fetcher.
func NewVideoService(videos VideoFetcher) *VideoService {
	return &VideoService{videos: videos}
}

// VideoService fetches video text over a video port.
type VideoService struct {
	videos VideoFetcher
}

// Fetch returns the video behind rawURL.
func (s *VideoService) Fetch(ctx context.Context, rawURL string) (*Video, error) {
	return s.videos.Fetch(ctx, rawURL)
}
