package schema

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/go-openapi/spec"
	"github.com/graphql-go/graphql"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// addRelationFields adds relation fields to schemas that contain *Ref fields
func (g *Gateway) addRelationFields(fields graphql.Fields, properties map[string]spec.Schema) {
	for fieldName := range properties {
		if !strings.HasSuffix(fieldName, "Ref") {
			continue
		}

		baseName := strings.TrimSuffix(fieldName, "Ref")
		sanitizedFieldName := sanitizeFieldName(fieldName)

		refField, exists := fields[sanitizedFieldName]
		if !exists {
			continue
		}

		enhancedType := g.enhanceRefTypeWithRelation(refField.Type, baseName)
		if enhancedType == nil {
			continue
		}

		fields[sanitizedFieldName] = &graphql.Field{
			Type: enhancedType,
		}
	}
}

// enhanceRefTypeWithRelation adds a relation field to a *Ref object type
func (g *Gateway) enhanceRefTypeWithRelation(originalType graphql.Output, baseName string) graphql.Output {
	objType, ok := originalType.(*graphql.Object)
	if !ok {
		return originalType
	}

	cacheKey := objType.Name() + "_" + baseName + "_Enhanced"
	if enhancedType, exists := g.enhancedTypesCache[cacheKey]; exists {
		return enhancedType
	}

	enhancedFields := g.copyOriginalFields(objType.Fields())
	g.addRelationField(enhancedFields, baseName)

	enhancedType := graphql.NewObject(graphql.ObjectConfig{
		Name:   sanitizeFieldName(cacheKey),
		Fields: enhancedFields,
	})

	g.enhancedTypesCache[cacheKey] = enhancedType
	return enhancedType
}

// copyOriginalFields converts FieldDefinition to Field for reuse
func (g *Gateway) copyOriginalFields(originalFieldDefs graphql.FieldDefinitionMap) graphql.Fields {
	enhancedFields := make(graphql.Fields, len(originalFieldDefs))
	for fieldName, fieldDef := range originalFieldDefs {
		enhancedFields[fieldName] = &graphql.Field{
			Type:        fieldDef.Type,
			Description: fieldDef.Description,
			Resolve:     fieldDef.Resolve,
		}
	}
	return enhancedFields
}

// addRelationField adds a single relation field to the enhanced fields
func (g *Gateway) addRelationField(enhancedFields graphql.Fields, baseName string) {
	targetType, targetGVK, ok := g.findRelationTarget(baseName)
	if !ok {
		return
	}

	sanitizedBaseName := sanitizeFieldName(baseName)
	enhancedFields[sanitizedBaseName] = &graphql.Field{
		Type:    targetType,
		Resolve: g.resolver.RelationResolver(baseName, *targetGVK),
	}
}

// findRelationTarget locates the GraphQL output type and its GVK for a relation target
func (g *Gateway) findRelationTarget(baseName string) (graphql.Output, *schema.GroupVersionKind, bool) {
	targetKind := cases.Title(language.English).String(baseName)

	for defKey, defSchema := range g.definitions {
		if g.matchesTargetKind(defSchema, targetKind) {
			// Resolve or build the GraphQL type
			var fieldType graphql.Output
			if existingType, exists := g.typesCache[defKey]; exists {
				fieldType = existingType
			} else {
				ft, _, err := g.convertSwaggerTypeToGraphQL(defSchema, defKey, []string{}, make(map[string]bool))
				if err != nil {
					continue
				}
				fieldType = ft
			}

			// Extract GVK from the schema definition
			gvk, err := g.getGroupVersionKind(defKey)
			if err != nil || gvk == nil {
				continue
			}

			return fieldType, gvk, true
		}
	}

	return nil, nil, false
}

// matchesTargetKind checks if a schema definition matches the target kind
func (g *Gateway) matchesTargetKind(defSchema spec.Schema, targetKind string) bool {
	gvkExt, ok := defSchema.Extensions["x-kubernetes-group-version-kind"]
	if !ok {
		return false
	}

	gvkSlice, ok := gvkExt.([]any)
	if !ok || len(gvkSlice) == 0 {
		return false
	}

	gvkMap, ok := gvkSlice[0].(map[string]any)
	if !ok {
		return false
	}

	kind, ok := gvkMap["kind"].(string)
	return ok && kind == targetKind
}
