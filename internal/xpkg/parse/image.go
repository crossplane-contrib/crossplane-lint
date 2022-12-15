package parse

import (
	"archive/tar"
	"bufio"
	"context"
	"io"
	"unicode"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg"
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg/fetch"
)

const (
	errMultipleAnnotatedLayers = "package is invalid due to multiple annotated base layers"
	errFetchLayer              = "failed to fetch annotated base layer from remote"
	errGetUncompressed         = "failed to get uncompressed contents from layer"
	errOpenPackageStream       = "failed to open package stream file"
)

const (
	layerAnnotation     = "io.crossplane.xpkg"
	baseAnnotationValue = "base"
	streamFile          = "package.yaml"
)

var _ PackageParser = &PackageImageParser{}

// PackageImageParser parses a package from an OCI image using a fetch.Fetcher
// as backend.
type PackageImageParser struct {
	fetcher fetch.Fetcher
}

// NewPackageImageParser creates a new PackageImageParser.
func NewPackageImageParser(fetcher fetch.Fetcher) *PackageImageParser {
	return &PackageImageParser{
		fetcher: fetcher,
	}
}

// ParsePackage from an OCI image. `pkg` is interpreted as the name OCI image
// name (i.e. `repo/image:tag`).
func (p *PackageImageParser) ParsePackage(pkg string) (*xpkg.Package, error) {
	ref, err := name.ParseReference(pkg)
	if err != nil {
		return nil, err
	}
	img, err := p.fetcher.Fetch(context.TODO(), ref)
	if err != nil {
		return nil, err
	}
	return p.getPackageFromImage(ref, img)
}

func (p *PackageImageParser) getPackageFromImage(ref name.Reference, img v1.Image) (*xpkg.Package, error) {
	// Copied from https://github.com/crossplane/crossplane/blob/eea7d35de8153e00e76a8eb98c2d46988e26f065/internal/controller/pkg/revision/imageback.go#L78
	manifest, err := img.Manifest()
	if err != nil {
		return nil, err
	}
	// Determine if the image is using annotated layers.
	var tarc io.ReadCloser
	defer func() {
		if tarc != nil {
			tarc.Close()
		}
	}()

	foundAnnotated := false
	for _, l := range manifest.Layers {
		if a, ok := l.Annotations[layerAnnotation]; !ok || a != baseAnnotationValue {
			continue
		}
		// NOTE(hasheddan): the xpkg specification dictates that only one layer
		// descriptor may be annotated as xpkg base. Since iterating through all
		// descriptors is relatively inexpensive, we opt to do so in order to
		// verify that we aren't just using the first layer annotated as xpkg
		// base.
		if foundAnnotated {
			return nil, errors.New(errMultipleAnnotatedLayers)
		}
		foundAnnotated = true
		layer, err := img.LayerByDigest(l.Digest)
		if err != nil {
			return nil, errors.Wrap(err, errFetchLayer)
		}
		tarc, err = layer.Uncompressed()
		if err != nil {
			return nil, errors.Wrap(err, errGetUncompressed)
		}
	}

	// If we still don't have content then we need to flatten image filesystem.
	if !foundAnnotated {
		tarc = mutate.Extract(img)
	}

	// The ReadCloser is an uncompressed tarball, either consisting of annotated
	// layer contents or flattened filesystem content. Either way, we only want
	// the package YAML stream.
	t := tar.NewReader(tarc)
	for {
		h, err := t.Next()
		if err != nil {
			return nil, errors.Wrap(err, errOpenPackageStream)
		}
		if h.Name == streamFile {
			break
		}
	}
	return parsePackage(t, ref.Name())
}

// parsePackage from a single file stream.
func parsePackage(reader io.Reader, sourceName string) (*xpkg.Package, error) {
	pkg := &xpkg.Package{
		Source: sourceName,
		Name:   sourceName,
	}
	yr := yaml.NewYAMLReader(bufio.NewReader(reader))
	for {
		data, err := yr.Read()
		if err != nil && !errors.Is(err, io.EOF) {
			return pkg, err
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if len(data) == 0 {
			continue
		}
		if isWhiteSpace(data) {
			continue
		}

		o := unstructured.Unstructured{}
		if err := yaml.Unmarshal(data, &o); err != nil {
			return nil, err
		}
		e := xpkg.PackageEntry{
			Object: o,
			Source: sourceName,
			Raw:    string(data),
		}
		pkg.Entries = append(pkg.Entries, e)
	}
	return pkg, nil
}

// isWhiteSpace determines whether the passed in bytes are all unicode white
// space.
func isWhiteSpace(bytes []byte) bool {
	empty := true
	for _, b := range bytes {
		if !unicode.IsSpace(rune(b)) {
			empty = false
			break
		}
	}
	return empty
}
