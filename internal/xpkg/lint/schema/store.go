package schema

import (
	"github.com/crossplane-contrib/crossplane-lint/internal/xpkg"
	"github.com/pkg/errors"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	errBuildCompositeCRD = "failed to build composite CRD"
	errBuildClaimCRD     = "failed to build claim CRD"
)

type SchemaStore struct {
	versions map[schema.GroupVersionKind]*extv1.CustomResourceDefinitionVersion
}

func NewSchemaStore() *SchemaStore {
	return &SchemaStore{
		versions: map[schema.GroupVersionKind]*extv1.CustomResourceDefinitionVersion{},
	}
}

func (s *SchemaStore) RegisterPackage(pkg *xpkg.Package) error {
	for _, e := range pkg.Entries {
		switch {
		case e.IsCRD():
			crd, err := e.AsCRD()
			if err != nil {
				return err
			}
			s.registerCRD(crd)
		case e.IsXRD():
			xrd, err := e.AsXRD()
			if err != nil {
				return err
			}
			comp, err := ForCompositeResource(xrd)
			if err != nil {
				return errors.Wrap(err, errBuildCompositeCRD)
			}
			s.registerCRD(comp)
			if xrd.Spec.ClaimNames != nil {
				claim, err := ForCompositeResourceClaim(xrd)
				if err != nil {
					return errors.Wrap(err, errBuildClaimCRD)
				}
				s.registerCRD(claim)
			}
		}
	}
	return nil
}

func (s *SchemaStore) registerCRD(crd *extv1.CustomResourceDefinition) {
	gk := schema.GroupKind{
		Group: crd.Spec.Group,
		Kind:  crd.Spec.Names.Kind,
	}
	for _, v := range crd.Spec.Versions {
		version := v
		gvk := gk.WithVersion(version.Name)
		addMetaDataToSchema(&version)
		s.versions[gvk] = &version
	}
}

func addMetaDataToSchema(crdv *extv1.CustomResourceDefinitionVersion) {
	additionalMetaProps := map[string]extv1.JSONSchemaProps{
		"name": {
			Type: "string",
		},
		"namespace": {
			Type: "string",
		},
		"labels": {
			AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
				Allows: true,
				Schema: &extv1.JSONSchemaProps{
					Type: "string",
				},
			},
		},
		"annotations": {
			AdditionalProperties: &extv1.JSONSchemaPropsOrBool{
				Allows: true,
				Schema: &extv1.JSONSchemaProps{
					Type: "string",
				},
			},
		},
	}
	if crdv.Schema == nil {
		crdv.Schema = &extv1.CustomResourceValidation{}
	}
	if crdv.Schema.OpenAPIV3Schema == nil {
		crdv.Schema.OpenAPIV3Schema = &extv1.JSONSchemaProps{
			Properties: map[string]extv1.JSONSchemaProps{},
		}
	}
	if _, exists := crdv.Schema.OpenAPIV3Schema.Properties["metadata"]; !exists {
		crdv.Schema.OpenAPIV3Schema.Properties["metadata"] = extv1.JSONSchemaProps{
			Type:       "object",
			Properties: map[string]extv1.JSONSchemaProps{},
		}
	}
	if crdv.Schema.OpenAPIV3Schema.Properties["metadata"].Properties == nil {
		prop := crdv.Schema.OpenAPIV3Schema.Properties["metadata"]
		prop.Properties = map[string]extv1.JSONSchemaProps{}
		crdv.Schema.OpenAPIV3Schema.Properties["metadata"] = prop
	}
	for name, prop := range additionalMetaProps {
		if _, exists := crdv.Schema.OpenAPIV3Schema.Properties["metadata"].Properties[name]; !exists {
			props := crdv.Schema.OpenAPIV3Schema.Properties["metadata"]
			props.Properties[name] = prop
			crdv.Schema.OpenAPIV3Schema.Properties["metadata"] = props
		}
	}
}

func (s *SchemaStore) GetCRDSchema(gvk schema.GroupVersionKind) *extv1.CustomResourceDefinitionVersion {
	version, exists := s.versions[gvk]
	if !exists {
		return nil
	}
	return version
}
