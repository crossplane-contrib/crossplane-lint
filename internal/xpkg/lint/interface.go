package lint

import (
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg"
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg/lint/jsonpath"
)

// Linter checks if a package for issues.
type Linter interface {
	// Lint pkg.
	Lint(pkg *xpkg.Package) LinterReport
}

type LinterReport struct {
	Issues []Issue
}

type Issue struct {
	RuleName    string
	Entry       *xpkg.PackageEntry
	Path        jsonpath.JSONPath
	PathValue   string
	Description string
}

type LinterContext interface {
	ReportIssue(issue Issue)
	GetCRDSchema(gvk schema.GroupVersionKind) *extv1.CustomResourceDefinitionVersion
}
