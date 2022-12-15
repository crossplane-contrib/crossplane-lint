package fetch

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Fetcher fetches package images.
type Fetcher interface {
	Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error)
	// Head(ctx context.Context, ref name.Reference) (*v1.Descriptor, error)
}
