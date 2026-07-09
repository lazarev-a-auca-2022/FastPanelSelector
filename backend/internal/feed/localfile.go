package feed

import (
	"context"
	"fmt"
	"os"
)

// LocalFileSource reads the feed from a configured path on disk — the
// simplest implementation of Source, matching today's known delivery
// mechanism (something external drops a fresh file at this path on a
// schedule; this service only consumes it).
type LocalFileSource struct {
	Path string
}

func NewLocalFileSource(path string) *LocalFileSource {
	return &LocalFileSource{Path: path}
}

func (s *LocalFileSource) Fetch(_ context.Context) (string, error) {
	if _, err := os.Stat(s.Path); err != nil {
		return "", fmt.Errorf("feed: local file %q not available: %w", s.Path, err)
	}
	return s.Path, nil
}
