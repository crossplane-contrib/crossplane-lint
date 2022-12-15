package fetch

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

const (
	errGetCached  = "failed to load cached image"
	errStoreCache = "failed to cache image"
)

var _ Fetcher = &FsCacheFetcher{}

type FsCacheFetcher struct {
	fetcher Fetcher
	fs      afero.Fs
}

func NewFsCacheFetcher(fs afero.Fs, wrapped Fetcher) *FsCacheFetcher {
	return &FsCacheFetcher{
		fs:      fs,
		fetcher: wrapped,
	}
}

func (f *FsCacheFetcher) Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error) {
	cached, err := f.getCached(ref)
	if err != nil {
		return nil, errors.Wrap(err, errGetCached)
	}
	if cached != nil {
		return cached, nil
	}

	// If not cached, fetch it and store it.
	img, err := f.fetcher.Fetch(ctx, ref)
	if err != nil {
		return nil, err
	}
	// Return the image anyway even if caching fails.
	return img, errors.Wrap(f.store(img, ref), errStoreCache)
}

func (f *FsCacheFetcher) getCached(ref name.Reference) (v1.Image, error) {
	fileName := cachedFileName(ref)
	exists, err := afero.Exists(f.fs, fileName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	opener := func() (io.ReadCloser, error) {
		return f.fs.Open(fileName)
	}
	return tarball.Image(opener, nil)
}

func (f *FsCacheFetcher) store(img v1.Image, ref name.Reference) error {
	fileName := cachedFileName(ref)
	tempFileName := fmt.Sprintf("%s.tmp", fileName)
	if err := f.fs.MkdirAll(filepath.Dir(fileName), 0755); err != nil {
		return err
	}
	file, err := f.fs.OpenFile(tempFileName, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if file != nil {
			file.Close()
		}
	}()
	if err := tarball.Write(ref, img, file); err != nil {
		return err
	}
	file.Close()
	file = nil
	return f.fs.Rename(tempFileName, fileName)
}

func cachedFileName(ref name.Reference) string {
	sum := md5.Sum([]byte(ref.Name()))
	return hex.EncodeToString(sum[:])
}
