package parse

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/crossplane-contrib/crossplane-lint/internal/config"
	"github.com/crossplane-contrib/crossplane-lint/internal/utils/sync"
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg"
)

const (
	errLoadPackages = "failed to load packages"
	errLoadPackage  = "failed to load package %s"
)

func LoadPackageDependencies(deps []config.PackageDescriptor, parser PackageParser) ([]*xpkg.Package, error) {
	resultChan, errorChan := loadPackageDependenciesConcurrently(deps, parser)
	loadedDeps, errs := sync.CollectWithError(resultChan, errorChan)
	for _, err := range errs {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	if len(errs) > 0 {
		return nil, errors.New(errLoadPackages)
	}
	return loadedDeps, nil
}

func loadPackageDependenciesConcurrently(deps []config.PackageDescriptor, parser PackageParser) (chan *xpkg.Package, chan error) {
	errorChan := make(chan error)
	resultChan := make(chan *xpkg.Package)
	eg := errgroup.Group{}
	for _, d := range deps {
		dep := d
		eg.Go(func() error {
			res, err := loadPackageDependency(dep, parser)
			if err != nil {
				errorChan <- err
			} else {
				resultChan <- res
			}
			return err
		})
	}

	go func() {
		_ = eg.Wait()
		close(errorChan)
		close(resultChan)
	}()
	return resultChan, errorChan
}

func loadPackageDependency(dep config.PackageDescriptor, parser PackageParser) (*xpkg.Package, error) {
	depPkg, err := parser.ParsePackage(dep.Image)
	return depPkg, errors.Wrapf(err, errLoadPackage, dep.Image)
}
