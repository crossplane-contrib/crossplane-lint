package rules

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg"
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg/lint"
)

type groupVersionKindName struct {
	gvk  schema.GroupVersionKind
	name string
}

// CheckDuplicateObjects checks if there are multiple objects in the package
// that share the same GroupVersionKind and name.
func CheckDuplicateObjects(ctx lint.LinterContext, pkg *xpkg.Package) {
	objects := map[groupVersionKindName][]*xpkg.PackageEntry{}
	for _, e := range pkg.Entries {
		gvkn := groupVersionKindName{
			gvk:  e.Object.GroupVersionKind(),
			name: e.Object.GetName(),
		}
		arr, exists := objects[gvkn]
		if exists {
			objects[gvkn] = append(arr, &e)
		} else {
			objects[gvkn] = []*xpkg.PackageEntry{&e}
		}
	}
	for gvkn, same := range objects {
		if len(same) > 1 {
			ctx.ReportIssue(lint.Issue{
				Entry:       same[0],
				Description: fmt.Sprintf("Multiple objects of kind '%s' with name '%s'", gvkn.gvk.String(), gvkn.name),
			})
		}
	}
}
