package xpkg

import (
	"reflect"

	xpv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	xppkgv1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	"github.com/pkg/errors"
	goyaml "gopkg.in/yaml.v3"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// Package represents the content of a Crossplane package.
type Package struct {
	// Source that was used to create this package.
	Source string

	// Name of this Package.
	Name string

	// Entries in this package.
	Entries []PackageEntry
}

// GetPackageDescriptor returns the first package descriptor in this Package or
// nil of there is none.
func (p *Package) GetPackageDescriptor() *PackageEntry {
	for _, obj := range p.Entries {
		if obj.IsPackageDescriptor() {
			return &obj
		}
	}
	return nil
}

type PackageEntry struct {
	// Object of this PackageEntry.
	Object unstructured.Unstructured

	// Source of this PackageEntry.
	Source string

	// The raw manifest of this PackageEntry.
	Raw string

	// Cached conversion of this PackageEntry.
	cachedConversion runtime.Object

	// Cached YAML node for JSON path evaluation.
	cachedNode *goyaml.Node
}

const (
	errPackageObjectNotType = "object is not of kind %s"
	errPackageObjectConvert = "failed to convert object to %s"
)

// IsXRD determines if e is an XRD.
func (e *PackageEntry) IsXRD() bool {
	return e.Object.GroupVersionKind() == xpv1.CompositeResourceDefinitionGroupVersionKind
}

// AsXRD returns the Object of e as a CompositeResourceDefinition.
func (e *PackageEntry) AsXRD() (*xpv1.CompositeResourceDefinition, error) {
	if !e.IsXRD() {
		return nil, errors.Errorf(errPackageObjectNotType, "XRD")
	}
	return convertToWithCache[*xpv1.CompositeResourceDefinition](e.Raw, &e.cachedConversion)
}

// IsComposition determines if e is a Composition.
func (e *PackageEntry) IsComposition() bool {
	return e.Object.GroupVersionKind() == xpv1.CompositionGroupVersionKind
}

// AsComposition returns the Object of e as a CompositeResourceDefinition.
func (e *PackageEntry) AsComposition() (*xpv1.Composition, error) {
	if !e.IsComposition() {
		return nil, errors.Errorf(errPackageObjectNotType, "Composition")
	}
	return convertToWithCache[*xpv1.Composition](e.Raw, &e.cachedConversion)
}

// Determines if e is either a Provider or Configuration package descriptor.
func (e *PackageEntry) IsPackageDescriptor() bool {
	gvk := e.Object.GroupVersionKind()
	return gvk == xppkgv1.ProviderGroupVersionKind || gvk == xppkgv1.ConfigurationGroupVersionKind
}

// AsPackageDescriptor returns the Object of e as a Pkg.
func (e *PackageEntry) AsPackageDescriptor() (xppkgv1.Pkg, error) {
	gvk := e.Object.GroupVersionKind()
	if gvk == xppkgv1.ProviderGroupVersionKind {
		return convertToWithCache[*xppkgv1.Provider](e.Raw, &e.cachedConversion)
	}
	if gvk == xppkgv1.ConfigurationGroupVersionKind {
		return convertToWithCache[*xppkgv1.Configuration](e.Raw, &e.cachedConversion)
	}
	return nil, errors.Errorf(errPackageObjectNotType, "package descriptor")
}

var (
	crdGVK = extv1.SchemeGroupVersion.WithKind("CustomResourceDefinition")
)

// IsCRD returns
func (e *PackageEntry) IsCRD() bool {
	gvk := e.Object.GroupVersionKind()
	return gvk == crdGVK
}

func (e *PackageEntry) AsCRD() (*extv1.CustomResourceDefinition, error) {
	if !e.IsCRD() {
		return nil, errors.Errorf(errPackageObjectNotType, "Composition")
	}
	return convertToWithCache[*extv1.CustomResourceDefinition](e.Raw, &e.cachedConversion)
}

// Converts Raw to T if cache is not of type T.
func convertToWithCache[T runtime.Object](raw string, cached *runtime.Object) (T, error) {
	if c, ok := (*cached).(T); ok {
		return c, nil
	}
	var res T
	if err := yaml.Unmarshal([]byte(raw), &res); err != nil {
		return res, errors.Wrapf(err, errPackageObjectConvert, reflect.TypeOf(res).Name())
	}
	return res, nil
}

// GetYamlNode for e.Raw.
func (e *PackageEntry) GetYamlNode() (*goyaml.Node, error) {
	if e.cachedNode != nil {
		return e.cachedNode, nil
	}
	node := &goyaml.Node{}
	if err := goyaml.Unmarshal([]byte(e.Raw), node); err != nil {
		return nil, err
	}
	e.cachedNode = node
	return node, nil
}
