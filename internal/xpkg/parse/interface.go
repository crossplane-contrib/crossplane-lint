package parse

import (
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg"
)

// PackageParser creates a new package from a supplied source.
type PackageParser interface {
	// ParsePackage from pkg.
	ParsePackage(pkg string) (*xpkg.Package, error)
}
