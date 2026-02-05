package types

import (
	"github.com/graphql-go/graphql"

	"k8s.io/kube-openapi/pkg/validation/spec"
)

// Converter handles OpenAPI to GraphQL type conversion.
// It uses a Registry for caching and recursion prevention.
type Converter struct {
	registry    *Registry
	definitions map[string]*spec.Schema
}

// NewConverter creates a new type converter with the given registry and definitions.
func NewConverter(registry *Registry, definitions map[string]*spec.Schema) *Converter {
	return &Converter{
		registry:    registry,
		definitions: definitions,
	}
}

// ConvertFields transforms schema properties to GraphQL fields and input fields.
func (c *Converter) ConvertFields(resourceScheme *spec.Schema, typePrefix string, fieldPath []string, processingTypes map[string]bool) (graphql.Fields, graphql.InputObjectConfigFieldMap, error) {
	fields := graphql.Fields{}
	inputFields := graphql.InputObjectConfigFieldMap{}

	for fieldName, fieldSpec := range resourceScheme.Properties {
		sanitizedFieldName := SanitizeFieldName(fieldName)
		currentFieldPath := append(fieldPath, fieldName)

		fieldType, inputFieldType, err := c.Convert(fieldSpec, typePrefix, currentFieldPath, processingTypes)
		if err != nil {
			return nil, nil, err
		}

		fields[sanitizedFieldName] = &graphql.Field{
			Type: fieldType,
		}

		inputFields[sanitizedFieldName] = &graphql.InputObjectFieldConfig{
			Type: inputFieldType,
		}
	}

	return fields, inputFields, nil
}

// Convert transforms an OpenAPI schema to GraphQL output and input types.
func (c *Converter) Convert(schema spec.Schema, typePrefix string, fieldPath []string, processingTypes map[string]bool) (graphql.Output, graphql.Input, error) {
	if len(schema.Type) == 0 {
		return c.handleRefType(schema, fieldPath, processingTypes)
	}

	switch schema.Type[0] {
	case "string":
		return graphql.String, graphql.String, nil
	case "integer":
		return graphql.Int, graphql.Int, nil
	case "number":
		return graphql.Float, graphql.Float, nil
	case "boolean":
		return graphql.Boolean, graphql.Boolean, nil
	case "array":
		return c.handleArrayType(schema, typePrefix, fieldPath, processingTypes)
	case "object":
		return c.handleObjectType(schema, typePrefix, fieldPath, processingTypes)
	default:
		// Handle unexpected types
		return graphql.String, graphql.String, nil
	}
}

// handleRefType handles $ref types (AllOf references).
func (c *Converter) handleRefType(schema spec.Schema, fieldPath []string, processingTypes map[string]bool) (graphql.Output, graphql.Input, error) {
	if len(schema.AllOf) == 0 {
		return graphql.String, graphql.String, nil
	}

	refKey := schema.AllOf[0].Ref.String()

	// Check if type is already being processed (recursion guard)
	if processingTypes[refKey] {
		// Return existing type to prevent infinite recursion
		if output, input := c.registry.Get(refKey); output != nil {
			return output, input, nil
		}
		// Return placeholder types to prevent recursion
		return graphql.String, graphql.String, nil
	}

	refDef, ok := c.definitions[refKey]
	if !ok {
		// Definition not found, return string
		return graphql.String, graphql.String, nil
	}

	// Mark as processing
	processingTypes[refKey] = true
	defer delete(processingTypes, refKey)

	fieldType, inputFieldType, err := c.Convert(*refDef, refKey, fieldPath, processingTypes)
	if err != nil {
		return nil, nil, err
	}

	// Store the types
	if objType, ok := fieldType.(*graphql.Object); ok {
		if inputObjType, inputOk := inputFieldType.(*graphql.InputObject); inputOk {
			c.registry.Register(refKey, objType, inputObjType)
		}
	}

	return fieldType, inputFieldType, nil
}

// handleArrayType handles array types.
func (c *Converter) handleArrayType(schema spec.Schema, typePrefix string, fieldPath []string, processingTypes map[string]bool) (graphql.Output, graphql.Input, error) {
	if schema.Items != nil && schema.Items.Schema != nil {
		itemType, inputItemType, err := c.Convert(*schema.Items.Schema, typePrefix, fieldPath, processingTypes)
		if err != nil {
			return nil, nil, err
		}
		return graphql.NewList(itemType), graphql.NewList(inputItemType), nil
	}
	return graphql.NewList(graphql.String), graphql.NewList(graphql.String), nil
}

// handleObjectType handles object types (nested objects, maps, empty objects).
func (c *Converter) handleObjectType(fieldSpec spec.Schema, typePrefix string, fieldPath []string, processingTypes map[string]bool) (graphql.Output, graphql.Input, error) {
	// Handle nested object with properties
	if len(fieldSpec.Properties) > 0 {
		return c.handleNestedObject(fieldSpec, typePrefix, fieldPath, processingTypes)
	}

	// Handle map types (additionalProperties)
	if fieldSpec.AdditionalProperties != nil && fieldSpec.AdditionalProperties.Schema != nil {
		if len(fieldSpec.AdditionalProperties.Schema.Type) == 1 && fieldSpec.AdditionalProperties.Schema.Type[0] == "string" {
			// This is a map[string]string
			return StringMapScalar, StringMapScalar, nil
		}
	}

	// Empty object, serialize as JSON string
	return JSONStringScalar, JSONStringScalar, nil
}

// handleNestedObject handles nested object types with properties.
func (c *Converter) handleNestedObject(fieldSpec spec.Schema, typePrefix string, fieldPath []string, processingTypes map[string]bool) (graphql.Output, graphql.Input, error) {
	typeName := GenerateTypeName(typePrefix, fieldPath)

	// Check if type already generated
	if output, input := c.registry.Get(typeName); output != nil {
		return output, input, nil
	}

	// Check if type is being processed (nil in cache)
	if c.registry.IsProcessing(typeName) {
		return graphql.String, graphql.String, nil
	}

	// Mark as processing to prevent recursion
	c.registry.MarkProcessing(typeName)

	nestedFields, nestedInputFields, err := c.ConvertFields(&fieldSpec, typeName, fieldPath, processingTypes)
	if err != nil {
		c.registry.UnmarkProcessing(typeName)
		return nil, nil, err
	}

	newType := graphql.NewObject(graphql.ObjectConfig{
		Name:   SanitizeFieldName(typeName),
		Fields: nestedFields,
	})

	newInputType := graphql.NewInputObject(graphql.InputObjectConfig{
		Name:   SanitizeFieldName(typeName) + "Input",
		Fields: nestedInputFields,
	})

	// Store the generated types
	c.registry.Register(typeName, newType, newInputType)

	return newType, newInputType, nil
}
