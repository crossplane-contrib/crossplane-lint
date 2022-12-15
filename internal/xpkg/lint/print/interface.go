package print

import (
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg/lint"
)

type Printer interface {
	PrintReport(report lint.LinterReport) error
}
