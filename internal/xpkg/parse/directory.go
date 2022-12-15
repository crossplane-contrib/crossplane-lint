package parse

import (
	"io/fs"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg"
)

const (
	errReadPackageDirectory = "failed to read package from directory"

	parserParallelism = 20
)

var _ PackageParser = &PackageDirectoryParser{}

// PackageDirectoryParser parses a package from a directory.
type PackageDirectoryParser struct {
	fs afero.Fs
}

// NewPackageDirectoryParser creates a new PackageDirectoryParser.
func NewPackageDirectoryParser(fs afero.Fs) *PackageDirectoryParser {
	return &PackageDirectoryParser{fs}

}

// ParsePackage from the files in the given directory.
func (p *PackageDirectoryParser) ParsePackage(directory string) (*xpkg.Package, error) {
	resultChan, errorChan := p.parseFilesConcurrently(directory)

	wg := sync.WaitGroup{}

	pkg := &xpkg.Package{}
	wg.Add(2)
	go func() {
		for res := range resultChan {
			pkg.Entries = append(pkg.Entries, res)
		}
		wg.Done()
	}()

	hasErrors := false
	go func() {
		for err := range errorChan {
			println(err.Error())
			hasErrors = true
		}
		wg.Done()
	}()

	wg.Wait()
	if hasErrors {
		return nil, errors.New(errReadPackageDirectory)
	}
	return pkg, nil
}

func (p *PackageDirectoryParser) parseFilesConcurrently(directory string) (chan xpkg.PackageEntry, chan error) {
	errChan := make(chan error)
	resChan := make(chan xpkg.PackageEntry)

	// Limit amout of parallel file handles to avoid syscall.ETOOMANYREFS on
	// macos.
	eg := errgroup.Group{}
	eg.SetLimit(parserParallelism)
	eg.Go(func() error {
		err := afero.Walk(p.fs, directory, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			ext := filepath.Ext(path)
			if !info.IsDir() && (ext == ".yaml" || ext == ".yml") {
				eg.Go(func() error {
					res, err := p.parseFile(path)
					if err != nil {
						errChan <- err
						return err
					}
					resChan <- res
					return nil
				})
			}
			return nil
		})
		if err != nil {
			errChan <- err
		}
		return err
	})

	go func() {
		_ = eg.Wait()
		close(errChan)
		close(resChan)
	}()
	return resChan, errChan
}

func (p *PackageDirectoryParser) parseFile(path string) (xpkg.PackageEntry, error) {
	raw, err := afero.ReadFile(p.fs, path)
	if err != nil {
		return xpkg.PackageEntry{}, err
	}

	o := unstructured.Unstructured{}
	if err := yaml.Unmarshal(raw, &o); err != nil {
		return xpkg.PackageEntry{}, err
	}
	return xpkg.PackageEntry{
		Object: o,
		Source: path,
		Raw:    string(raw),
	}, nil
}
