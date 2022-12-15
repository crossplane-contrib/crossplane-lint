package rules

import (
	xpv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg"
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg/lint"
)

const (
	errConvertTo = "failed to convert object to %s"

	errParseGroupVersion       = "failed to convert object to Composition"
	errNoMatchingCompositeType = "no composite type found for %s.%s/%s"
)

// CheckCompositionCompositeTypeRef checks if a composition in manifest points
// to a valid composition.
func CheckCompositionCompositeTypeRef(ctx lint.LinterContext, pkg *xpkg.Package) {
	for _, manifest := range pkg.Entries {
		if !manifest.IsComposition() {
			continue
		}
		comp, err := manifest.AsComposition()
		if err != nil {
			ctx.ReportIssue(lint.Issue{
				Entry:       &manifest,
				Description: errors.Wrapf(err, errConvertTo, "Composition").Error(),
			})
			continue
		}
		gv, err := schema.ParseGroupVersion(comp.Spec.CompositeTypeRef.APIVersion)
		if err != nil {
			ctx.ReportIssue(lint.Issue{
				Entry:       &manifest,
				Description: errors.Wrap(err, errParseGroupVersion).Error(),
			})
			continue
		}
		gvk := gv.WithKind(comp.Spec.CompositeTypeRef.Kind)
		xrd, issue := getCompositeXRD(pkg, gvk)
		if issue != nil {
			ctx.ReportIssue(*issue)
		} else if xrd == nil {
			ctx.ReportIssue(lint.Issue{
				Entry:       &manifest,
				Description: errors.Errorf(errNoMatchingCompositeType, gvk.Kind, gvk.Group, gvk.Version).Error(),
			})
		}
	}
}

func getCompositeXRD(pkg *xpkg.Package, compositeGVK schema.GroupVersionKind) (*xpv1.CompositeResourceDefinitionVersion, *lint.Issue) {
	for _, e := range pkg.Entries {
		if !e.IsXRD() {
			continue
		}
		xrd, err := e.AsXRD()
		if err != nil {
			return nil, &lint.Issue{
				Entry:       &e,
				Description: errors.Wrapf(err, errConvertTo, "XRD").Error(),
			}
		}
		if xrd.Spec.Group == compositeGVK.Group && xrd.Spec.Names.Kind == compositeGVK.Kind {
			for _, v := range xrd.Spec.Versions {
				if v.Name == compositeGVK.Version {
					return &v, nil
				}
			}
		}
	}
	return nil, nil
}
