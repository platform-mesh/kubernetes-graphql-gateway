package resolver

import (
	"context"
	"strings"

	"github.com/graphql-go/graphql"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// referenceInfo holds extracted reference details
type referenceInfo struct {
	name      string
	namespace string
	kind      string
	apiGroup  string
}

// RelationResolver creates a GraphQL resolver for relation fields
// Relationships are only enabled for GetItem queries to prevent N+1 problems in ListItems and Subscriptions
func (r *Service) RelationResolver(fieldName string, gvk schema.GroupVersionKind) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		// Determine operation type from GraphQL path analysis
		operation := r.detectOperationFromGraphQLInfo(p)

		r.log.Debug().
			Str("fieldName", fieldName).
			Str("operation", operation).
			Str("graphqlField", p.Info.FieldName).
			Msg("RelationResolver called")

		// Check if relationships are allowed in this query context
		if !r.isRelationResolutionAllowedForOperation(operation) {
			r.log.Debug().
				Str("fieldName", fieldName).
				Str("operation", operation).
				Msg("Relationship resolution disabled for this operation type")
			return nil, nil
		}

		parentObj, ok := p.Source.(map[string]any)
		if !ok {
			return nil, nil
		}

		refInfo := r.extractReferenceInfo(parentObj, fieldName)
		if refInfo.name == "" {
			return nil, nil
		}

		return r.resolveReference(p.Context, refInfo, gvk)
	}
}

// extractReferenceInfo extracts reference details from a *Ref object
func (r *Service) extractReferenceInfo(parentObj map[string]any, fieldName string) referenceInfo {
	name, _ := parentObj["name"].(string)
	if name == "" {
		return referenceInfo{}
	}

	namespace, _ := parentObj["namespace"].(string)
	apiGroup, _ := parentObj["apiGroup"].(string)

	kind, _ := parentObj["kind"].(string)
	if kind == "" {
		// Fallback: infer kind from field name (e.g., "role" -> "Role")
		kind = cases.Title(language.English).String(fieldName)
	}

	return referenceInfo{
		name:      name,
		namespace: namespace,
		kind:      kind,
		apiGroup:  apiGroup,
	}
}

// resolveReference fetches a referenced Kubernetes resource using strict conflict resolution
func (r *Service) resolveReference(ctx context.Context, ref referenceInfo, targetGVK schema.GroupVersionKind) (interface{}, error) {
	// Use provided reference info to override GVK if specified
	finalGVK := targetGVK
	if ref.apiGroup != "" {
		finalGVK.Group = ref.apiGroup
	}
	if ref.kind != "" {
		finalGVK.Kind = ref.kind
	}

	// Convert sanitized group to original before calling the client
	finalGVK.Group = r.getOriginalGroupName(finalGVK.Group)

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(finalGVK)

	key := client.ObjectKey{Name: ref.name}
	if ref.namespace != "" {
		key.Namespace = ref.namespace
	}

	if err := r.runtimeClient.Get(ctx, key, obj); err != nil {
		// For "not found" errors, return nil to allow graceful degradation
		// This handles cases where referenced resources are deleted or don't exist
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		// For other errors (network, permission, etc.), log and return the actual error
		// This ensures proper error propagation for debugging and monitoring
		r.log.Error().
			Err(err).
			Str("operation", "resolve_relation").
			Str("group", finalGVK.Group).
			Str("version", finalGVK.Version).
			Str("kind", finalGVK.Kind).
			Str("name", ref.name).
			Str("namespace", ref.namespace).
			Msg("Unable to resolve referenced object")
		return nil, err
	}

	// Happy path: resource found successfully
	return obj.Object, nil
}

// isRelationResolutionAllowedForOperation checks if relationship resolution should be enabled for the given operation type
func (r *Service) isRelationResolutionAllowedForOperation(operation string) bool {
	// Only allow relationships for GetItem and GetItemAsYAML operations
	switch operation {
	case GET_ITEM, GET_ITEM_AS_YAML:
		return true
	case LIST_ITEMS, SUBSCRIBE_ITEM, SUBSCRIBE_ITEMS:
		return false
	default:
		// For unknown operations, be conservative and disable relationships
		r.log.Debug().Str("operation", operation).Msg("Unknown operation type, disabling relationships")
		return false
	}
}

// detectOperationFromGraphQLInfo analyzes GraphQL field path to determine operation type
// This looks at the parent field context to determine if we're in a list, single item, or subscription
func (r *Service) detectOperationFromGraphQLInfo(p graphql.ResolveParams) string {
	if p.Info.Path == nil {
		return "unknown"
	}

	// Walk up the path to find the parent resolver context
	path := p.Info.Path
	for path.Prev != nil {
		path = path.Prev

		// Check if we find a parent field that indicates the operation type
		if fieldName, ok := path.Key.(string); ok {
			fieldLower := strings.ToLower(fieldName)

			// Check for subscription patterns
			if strings.Contains(fieldLower, "subscription") {
				r.log.Debug().
					Str("parentField", fieldName).
					Msg("Detected subscription context from parent field")
				return SUBSCRIBE_ITEMS
			}

			// Check for mutation patterns
			if strings.HasPrefix(fieldLower, "create") {
				return CREATE_ITEM
			}
			if strings.HasPrefix(fieldLower, "update") {
				return UPDATE_ITEM
			}
			if strings.HasPrefix(fieldLower, "delete") {
				return DELETE_ITEM
			}

			// Check for YAML patterns
			if strings.HasSuffix(fieldLower, "yaml") {
				return GET_ITEM_AS_YAML
			}

			// Check for list patterns (plural without args, or explicitly plural fields)
			if strings.HasSuffix(fieldName, "s") && !strings.HasSuffix(fieldName, "Status") {
				// This looks like a plural field, likely a list operation
				r.log.Debug().
					Str("parentField", fieldName).
					Msg("Detected list context from parent field")
				return LIST_ITEMS
			}
		}
	}

	// If we can't determine from parent context, assume it's a single item operation
	// This is the safe default that allows relationships for queries
	r.log.Debug().
		Str("currentField", p.Info.FieldName).
		Msg("Could not determine operation type from GraphQL path, defaulting to GetItem (enables relations)")
	return GET_ITEM
}
