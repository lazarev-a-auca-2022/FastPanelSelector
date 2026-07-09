// Package feed abstracts how the scheduler gets hold of today's plan feed
// file, so the "how" (local path today; HTTP/SFTP/S3 later) is a contained,
// swappable concern.
package feed

import "context"

// Source returns the local filesystem path to the current feed file, ready
// to be opened and parsed.
type Source interface {
	Fetch(ctx context.Context) (path string, err error)
}
