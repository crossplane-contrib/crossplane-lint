package rules

import (
	"sync"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	xpv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/pkg/errors"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg"
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg/lint"
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg/lint/jsonpath"
)

type scopedContext struct {
	linterContext lint.LinterContext
	entry         *xpkg.PackageEntry
	basePath      jsonpath.JSONPath
}

func (c *scopedContext) ReportIssueRequireField(path jsonpath.JSONPath) {
	c.linterContext.ReportIssue(lint.Issue{
		Entry:       c.entry,
		Description: "require field",
		Path:        jsonpath.NewJSONPath(c.basePath, path),
	})
}

func (c *scopedContext) ReportIssueFieldPath(path jsonpath.JSONPath, description, pathValue string) {
	c.linterContext.ReportIssue(lint.Issue{
		Entry:       c.entry,
		Description: description,
		Path:        jsonpath.NewJSONPath(c.basePath, path),
		PathValue:   pathValue,
	})
}

func (c *scopedContext) Wrap(path jsonpath.JSONPath) scopedContext {
	return scopedContext{
		entry:         c.entry,
		linterContext: c.linterContext,
		basePath:      jsonpath.NewJSONPath(c.basePath, path),
	}
}

func CheckCompositionFieldPaths(ctx lint.LinterContext, pkg *xpkg.Package) {
	wg := sync.WaitGroup{}
	wg.Add(len(pkg.Entries))

	for _, m := range pkg.Entries {
		manifest := m
		go func() {
			checkCompositionFieldPaths(ctx, pkg, manifest)
			wg.Done()
		}()
	}
	wg.Wait()
}

func checkCompositionFieldPaths(ctx lint.LinterContext, pkg *xpkg.Package, manifest xpkg.PackageEntry) {
	if !manifest.IsComposition() {
		return
	}
	comp, err := manifest.AsComposition()
	if err != nil {
		ctx.ReportIssue(lint.Issue{
			Entry:       &manifest,
			Description: errors.Wrapf(err, errConvertTo, "Composition").Error(),
		})
		return
	}
	compositeGv, err := schema.ParseGroupVersion(comp.Spec.CompositeTypeRef.APIVersion)
	if err != nil {
		ctx.ReportIssue(lint.Issue{
			Entry:       &manifest,
			Description: errors.Wrap(err, errParseGroupVersion).Error(),
		})
		return
	}
	compositeGvk := compositeGv.WithKind(comp.Spec.CompositeTypeRef.Kind)
	compositeCRD := ctx.GetCRDSchema(compositeGvk)
	if compositeCRD == nil {
		ctx.ReportIssue(lint.Issue{
			Entry:       &manifest,
			Path:        jsonpath.NewJSONPath("spec", "compositeTypeRef"),
			Description: errors.Errorf(errNoCRDForGVK, compositeGvk.String()).Error(),
		})
		return
	}

	for i, r := range comp.Spec.Resources {
		base := &unstructured.Unstructured{}
		if err := yaml.Unmarshal(r.Base.Raw, base); err != nil {
			ctx.ReportIssue(lint.Issue{
				Entry:       &manifest,
				Path:        jsonpath.NewJSONPath("spec", "resources", i, "base"),
				Description: errors.Wrap(err, "failed to parse base").Error(),
			})
			continue
		}
		baseGvk := base.GetObjectKind().GroupVersionKind()
		baseCrd := ctx.GetCRDSchema(baseGvk)
		if baseCrd == nil {
			ctx.ReportIssue(lint.Issue{
				Entry:       &manifest,
				Path:        jsonpath.NewJSONPath("spec", "resources", i, "base"),
				Description: errors.Errorf(errNoCRDForGVK, baseGvk.String()).Error(),
			})
			continue
		}
		for ip, p := range r.Patches {
			sctx := scopedContext{
				linterContext: ctx,
				entry:         &manifest,
				basePath:      jsonpath.NewJSONPath("spec", "resources", i, "patches", ip),
			}
			validatePatch(sctx, p, comp, compositeGvk, baseGvk)
		}
	}
}

func validatePatch(ctx scopedContext, p xpv1.Patch, comp *xpv1.Composition, compositeGvk, baseGvk schema.GroupVersionKind) {
	switch p.Type {
	case xpv1.PatchTypeCombineToComposite:
		validateCombinePatch(ctx, p, baseGvk, compositeGvk)
	case xpv1.PatchTypeCombineFromComposite:
		validateCombinePatch(ctx, p, compositeGvk, baseGvk)
	case xpv1.PatchTypeToCompositeFieldPath:
		validateSinglePatch(ctx, p, baseGvk, compositeGvk)
	case "", xpv1.PatchTypeFromCompositeFieldPath:
		validateSinglePatch(ctx, p, compositeGvk, baseGvk)
	case xpv1.PatchTypeFromEnvironmentFieldPath:
		validateEnvironmentPatch(ctx, p, compositeGvk, baseGvk)
	case xpv1.PatchTypePatchSet:
		if p.PatchSetName == nil {
			ctx.ReportIssueRequireField(jsonpath.NewJSONPath("patchSetName"))
			break
		}
		var patchSet *xpv1.PatchSet
		var patchSetIndex int
		for i, s := range comp.Spec.PatchSets {
			if s.Name == *p.PatchSetName {
				patchSet = &s
				patchSetIndex = i
				break
			}
		}
		if patchSet == nil {
			ctx.ReportIssueFieldPath(jsonpath.NewJSONPath("patchSetName"), "no matching patchset found", *p.PatchSetName)
			break
		}
		validatePatchSet(ctx, *patchSet, compositeGvk, baseGvk, jsonpath.NewJSONPath("#inlined", "spec", "patchSets", patchSetIndex))
	default:
		ctx.ReportIssueFieldPath(jsonpath.NewJSONPath("type"), "unknown patch type", string(p.Type))
	}
}

func validateCombinePatch(ctx scopedContext, p xpv1.Patch, fromGvk, toGvk schema.GroupVersionKind) {
	if p.Combine == nil || len(p.Combine.Variables) == 0 {
		ctx.ReportIssueRequireField(jsonpath.NewJSONPath("combine", "variables"))
	} else {
		for i, v := range p.Combine.Variables {
			if v.FromFieldPath == "" {
				ctx.ReportIssueRequireField(jsonpath.NewJSONPath("combine", "variables", i, "fromFieldPath"))
				continue
			}
			if err := validateFieldPath(ctx, fromGvk, v.FromFieldPath); err != nil {
				ctx.ReportIssueFieldPath(jsonpath.NewJSONPath("combine", "variables", i, "fromFieldPath"), err.Error(), v.FromFieldPath)
			}
		}
	}
	if p.ToFieldPath == nil {
		ctx.ReportIssueRequireField(jsonpath.NewJSONPath("toFieldPath"))
		return
	}
	if err := validateFieldPath(ctx, toGvk, *p.ToFieldPath); err != nil {
		ctx.ReportIssueFieldPath(jsonpath.NewJSONPath("toFieldPath"), err.Error(), *p.ToFieldPath)
	}
}

func validateEnvironmentPatch(ctx scopedContext, p xpv1.Patch, fromGvk, toGvk schema.GroupVersionKind) {
	if p.FromFieldPath == nil {
		ctx.ReportIssueRequireField(jsonpath.NewJSONPath("fromFieldPath"))
	}
	// TODO: how to validate FromFieldPath?
	toFieldPath := p.ToFieldPath
	if toFieldPath == nil {
		toFieldPath = p.FromFieldPath
	}
	if toFieldPath == nil {
		ctx.ReportIssueRequireField(jsonpath.NewJSONPath("toFieldPath"))
	} else if err := validateFieldPath(ctx, toGvk, *toFieldPath); err != nil {
		ctx.ReportIssueFieldPath(jsonpath.NewJSONPath("toFieldPath"), err.Error(), *toFieldPath)
	}
}

func validateSinglePatch(ctx scopedContext, p xpv1.Patch, fromGvk, toGvk schema.GroupVersionKind) {
	if p.FromFieldPath == nil {
		ctx.ReportIssueRequireField(jsonpath.NewJSONPath("fromFieldPath"))
	} else if err := validateFieldPath(ctx, fromGvk, *p.FromFieldPath); err != nil {
		ctx.ReportIssueFieldPath(jsonpath.NewJSONPath("fromFieldPath"), err.Error(), *p.FromFieldPath)
	}
	toFieldPath := p.ToFieldPath
	if toFieldPath == nil {
		toFieldPath = p.FromFieldPath
	}
	if toFieldPath == nil {
		ctx.ReportIssueRequireField(jsonpath.NewJSONPath("toFieldPath"))
	} else if err := validateFieldPath(ctx, toGvk, *toFieldPath); err != nil {
		ctx.ReportIssueFieldPath(jsonpath.NewJSONPath("toFieldPath"), err.Error(), *toFieldPath)
	}
}

func validatePatchSet(ctx scopedContext, ps xpv1.PatchSet, compositeGvk, baseGvk schema.GroupVersionKind, patchPath jsonpath.JSONPath) {
	for i, p := range ps.Patches {
		sctx := ctx.Wrap(jsonpath.NewJSONPath(patchPath, "patches", i, "type"))
		switch p.Type {
		case xpv1.PatchTypeCombineToComposite:
			validateCombinePatch(sctx, p, baseGvk, compositeGvk)
		case xpv1.PatchTypeCombineFromComposite:
			validateCombinePatch(sctx, p, compositeGvk, baseGvk)
		case xpv1.PatchTypeToCompositeFieldPath:
			validateSinglePatch(sctx, p, baseGvk, compositeGvk)
		case "", xpv1.PatchTypeFromCompositeFieldPath:
			validateSinglePatch(sctx, p, compositeGvk, baseGvk)
		case xpv1.PatchTypePatchSet:
			sctx.ReportIssueFieldPath(jsonpath.NewJSONPath("type"), "nested patch sets are not allowed", string(p.Type))
		case xpv1.PatchTypeFromEnvironmentFieldPath:
			validateEnvironmentPatch(sctx, p, compositeGvk, baseGvk)
		default:
			sctx.ReportIssueFieldPath(jsonpath.NewJSONPath("type"), "unknown patch type", string(p.Type))
		}
	}
}

const (
	errValidateFieldPath  = "failed to validate segment %d"
	errNoCRDForGVK        = "no CRD for %s"
	errFieldPathWrongType = "expected type '%s' but got '%s'"
	errFieldNotFound      = "property '%s' not found"
	errFieldArrayNoItems  = "prop type is array but missing items definition"
	errInvalidSegmentType = "invalid segment type %d"
)

func validateFieldPath(ctx scopedContext, gvk schema.GroupVersionKind, rawPath string) error {
	path, err := fieldpath.Parse(rawPath)
	if err != nil {
		return err
	}
	crd := ctx.linterContext.GetCRDSchema(gvk)
	if crd == nil {
		return errors.Errorf(errNoCRDForGVK, gvk.String())
	}
	current := crd.Schema.OpenAPIV3Schema
	for _, segment := range path {
		if current == nil {
			return nil
		}
		var err error
		current, err = validateFieldPathSegment(current, segment)
		if err != nil { // Workaround for now
			return err
		}
	}
	return nil
}

func validateFieldPathSegment(current *extv1.JSONSchemaProps, segment fieldpath.Segment) (*extv1.JSONSchemaProps, error) {
	switch segment.Type {
	case fieldpath.SegmentField:
		propType := current.Type
		if propType == "" {
			propType = "object"
		}
		if propType != "object" {
			return nil, errors.Errorf(errFieldPathWrongType, "object", propType)
		}
		if pointer.BoolDeref(current.XPreserveUnknownFields, false) {
			return nil, nil
		}
		prop, exists := current.Properties[segment.Field]
		if !exists {
			if current.AdditionalProperties != nil && current.AdditionalProperties.Allows {
				return current.AdditionalProperties.Schema, nil
			}
			return nil, errors.Errorf(errFieldNotFound, segment.Field)
		}
		return &prop, nil
	case fieldpath.SegmentIndex:
		if current.Type != "array" {
			return nil, errors.Errorf(errFieldPathWrongType, "array", current.Type)
		}
		// TOOD: We currently don't support multiple item schemas.
		if current.Items == nil || current.Items.Schema == nil {
			return nil, errors.New(errFieldArrayNoItems)
		}
		return current.Items.Schema, nil
	}
	return nil, errors.Errorf(errInvalidSegmentType, segment.Type)
}
