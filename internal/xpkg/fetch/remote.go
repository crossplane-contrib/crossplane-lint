package fetch

import (
	"context"
	"io"
	"net/http"

	ecr "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

var (
	amazonKeychain authn.Keychain = authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(io.Discard)))
)

var _ Fetcher = &RemoteFetcher{}

// RemoteFetcher uses default and AWS credentials to connect to a registry.
type RemoteFetcher struct {
	transport http.RoundTripper
}

// NewRemoteFetcher creates a new RemoteFetcher.
func NewRemoteFetcher() *RemoteFetcher {
	return &RemoteFetcher{
		transport: remote.DefaultTransport.Clone(),
	}
}

// Fetch fetches a package image.
func (i *RemoteFetcher) Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error) {
	auth := authn.NewMultiKeychain(
		authn.DefaultKeychain,
		amazonKeychain,
	)
	return remote.Image(ref, remote.WithAuthFromKeychain(auth), remote.WithTransport(i.transport), remote.WithContext(ctx))
}
