package resolver

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/platform-mesh/golang-commons/sentry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/graphql-go/graphql/language/ast"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/graphql-go/graphql"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrFailedToCastEventObjectToUnstructured = fmt.Errorf("failed to cast event object to unstructured")
)

func (r *Service) SubscribeItem(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		_, span := otel.Tracer("").Start(p.Context, "SubscribeItem", trace.WithAttributes(attribute.String("kind", gvk.Kind)))
		defer span.End()
		resultChannel := make(chan interface{})
		go r.runWatch(p, gvk, resultChannel, true, scope)

		return resultChannel, nil
	}
}

func (r *Service) SubscribeItems(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		_, span := otel.Tracer("").Start(p.Context, "SubscribeItems", trace.WithAttributes(attribute.String("kind", gvk.Kind)))
		defer span.End()
		resultChannel := make(chan interface{})
		go r.runWatch(p, gvk, resultChannel, false, scope)

		return resultChannel, nil
	}
}

func (r *Service) runWatch(
	p graphql.ResolveParams,
	gvk schema.GroupVersionKind,
	resultChannel chan interface{},
	singleItem bool,
	scope v1.ResourceScope,
) {
	defer close(resultChannel)

	ctx := p.Context

	gvk.Group = r.getOriginalGroupName(gvk.Group)

	labelSelector, err := getStringArg(p.Args, LabelSelectorArg, false)
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to get label selector argument")
		resultChannel <- errors.Wrap(err, "failed to get label selector argument")
		return
	}

	subscribeToAll, err := getBoolArg(p.Args, SubscribeToAllArg, false)
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to get subscribeToAll argument")
		resultChannel <- errors.Wrap(err, "failed to get subscribeToAll argument")
		return
	}

	fieldsToWatch := extractRequestedFields(p.Info)

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List",
	})

	var opts []client.ListOption

	var namespace string
	if isResourceNamespaceScoped(scope) {
		isNamespaceRequired := singleItem
		namespace, err = getStringArg(p.Args, NamespaceArg, isNamespaceRequired)
		if err != nil {
			r.log.Error().Err(err).Msg("Failed to get namespace argument")
			resultChannel <- errors.Wrap(err, "failed to get namespace argument")
			return
		}
		if namespace != "" {
			opts = append(opts, client.InNamespace(namespace))
		}
	}

	if labelSelector != "" {
		selector, err := labels.Parse(labelSelector)
		if err != nil {
			r.log.Error().Err(err).Str("labelSelector", labelSelector).Msg("Invalid label selector")
			resultChannel <- errors.Wrap(err, "invalid label selector")
			return
		}
		opts = append(opts, client.MatchingLabelsSelector{Selector: selector})
	}

	var name string
	if singleItem {
		name, err = getStringArg(p.Args, NameArg, true)
		if err != nil {
			r.log.Error().Err(err).Msg("Failed to get name argument")
			resultChannel <- errors.Wrap(err, "failed to get name argument")
			return
		}
		opts = append(opts, client.MatchingFields{"metadata.name": name})
	}

	sortBy, err := getStringArg(p.Args, SortByArg, false)
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to get sortBy argument")
		resultChannel <- errors.Wrap(err, "failed to get sortBy argument")
		return
	}

	if !singleItem {
		select {
		case <-ctx.Done():
			return
		case resultChannel <- []map[string]interface{}{}:
		}
	}

	listResult := &unstructured.UnstructuredList{}
	listResult.SetGroupVersionKind(schema.GroupVersionKind{
		Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List",
	})
	r.runtimeClient.List(ctx, listResult, opts...)

	rv := listResult.GetResourceVersion()
	lo := &client.ListOptions{
		Raw: &metav1.ListOptions{
			ResourceVersion: rv,
		},
	}
	opts = append(opts, lo)

	watcher, err := r.runtimeClient.Watch(ctx, list, opts...)
	if err != nil {
		r.log.Error().Err(err).Str("gvk", gvk.String()).Msg("Failed to start watch")

		sentry.CaptureError(err, sentry.Tags{"namespace": namespace}, sentry.Extras{"gvk": gvk.String()})

		resultChannel <- errors.Wrap(err, "failed to start watch")
		return
	}
	defer watcher.Stop()

	previousObjects := make(map[string]*unstructured.Unstructured)
	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return
			}
			obj, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				err = ErrFailedToCastEventObjectToUnstructured
				r.log.Error().Err(err)

				sentry.CaptureError(err, sentry.Tags{"namespace": namespace})

				resultChannel <- errors.Wrap(err, "failed to cast event object to unstructured")
				return
			}
			key := obj.GetNamespace() + "/" + obj.GetName()

			var sendUpdate bool
			switch event.Type {
			case watch.Added:
				previousObjects[key] = obj.DeepCopy()
				sendUpdate = true
			case watch.Modified:
				oldObj := previousObjects[key]
				if subscribeToAll {
					sendUpdate = true
				} else {
					var changed bool
					changed, err = determineFieldChanged(oldObj, obj, fieldsToWatch)
					if err != nil {
						r.log.Error().Err(err).Msg("Failed to determine field changes")

						sentry.CaptureError(err, sentry.Tags{"namespace": namespace})

						resultChannel <- errors.Wrap(err, "failed to determine field changed")
						return
					}
					sendUpdate = changed
				}
				previousObjects[key] = obj.DeepCopy()
			case watch.Deleted:
				delete(previousObjects, key)
				sendUpdate = true
			}

			if sendUpdate {
				if singleItem {
					var singleObj *unstructured.Unstructured
					if name != "" {
						singleObj = previousObjects[namespace+"/"+name]
					}

					var data interface{}
					if singleObj != nil { // object can be nil in case it is deleted
						data = singleObj.Object
					}

					select {
					case <-ctx.Done():
						return
					case resultChannel <- data:
					}
				} else {
					items := make([]unstructured.Unstructured, 0, len(previousObjects))
					for _, item := range previousObjects {
						items = append(items, *item.DeepCopy())
					}

					err = validateSortBy(items, sortBy)
					if err != nil {
						r.log.Error().Err(err).Str(SortByArg, sortBy).Msg("Invalid sortBy field path")
						resultChannel <- errors.Wrap(err, "invalid sortBy field path")
						return
					}

					sort.Slice(items, func(i, j int) bool {
						return compareUnstructured(items[i], items[j], sortBy) < 0
					})

					sortedItems := make([]map[string]any, len(items))
					for i, item := range items {
						sortedItems[i] = item.Object
					}

					select {
					case <-ctx.Done():
						return
					case resultChannel <- sortedItems:
					}
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// extractRequestedFields uses p.Info to determine the fields requested by the client.
// It returns a slice of strings representing the "paths" of requested fields.
func extractRequestedFields(info graphql.ResolveInfo) []string {
	var fields []string
	for _, fieldAST := range info.FieldASTs {
		fields = append(fields, parseSelectionSet(fieldAST.SelectionSet, "")...)
	}
	return fields
}

// parseSelectionSet recursively extracts field paths from a selection set.
// If `prefix` is non-empty, it prefixes subfields with `prefix + "."`.
func parseSelectionSet(selectionSet *ast.SelectionSet, prefix string) []string {
	var result []string
	if selectionSet == nil {
		return result
	}

	for _, selection := range selectionSet.Selections {
		switch sel := selection.(type) {
		case *ast.Field:
			fieldName := sel.Name.Value
			fullPath := fieldName
			if prefix != "" {
				fullPath = prefix + "." + fieldName
			}

			// If this field has a sub-selection set, recurse
			if sel.SelectionSet != nil && len(sel.SelectionSet.Selections) > 0 {
				subFields := parseSelectionSet(sel.SelectionSet, fullPath)
				result = append(result, subFields...)
			} else {
				// Leaf field
				result = append(result, fullPath)
			}
		}
	}
	return result
}

func determineFieldChanged(oldObj, newObj *unstructured.Unstructured, fields []string) (bool, error) {
	if oldObj == nil {
		// No previous object, so treat as changed
		return true, nil
	}

	for _, fieldPath := range fields {
		oldValue, foundOld, err := getFieldValue(oldObj, fieldPath)
		if err != nil {
			return false, err
		}
		newValue, foundNew, err := getFieldValue(newObj, fieldPath)
		if err != nil {
			return false, err
		}
		if !foundOld && !foundNew {
			// Field not present in both, consider no change
			continue
		}
		if !foundOld || !foundNew {
			// Field present in one but not the other, so changed
			return true, nil
		}
		if !reflect.DeepEqual(oldValue, newValue) {
			// Field value has changed
			return true, nil
		}
	}

	return false, nil
}

// Helper function to get the value of a field from an unstructured object
func getFieldValue(obj *unstructured.Unstructured, fieldPath string) (interface{}, bool, error) {
	fields := strings.Split(fieldPath, ".")
	var current interface{} = obj.Object

	for i, field := range fields {
		switch v := current.(type) {
		case map[string]interface{}:
			value, found, err := unstructured.NestedFieldNoCopy(v, field)
			if err != nil {
				return nil, false, fmt.Errorf("error accessing field %s: %v", strings.Join(fields[:i+1], "."), err)
			}
			if !found {
				return nil, false, nil
			}
			current = value
		case []interface{}:
			// in case of slice, we return it, and that slice will be compared later using deep equal
			return current, true, nil
		default:
			return nil, false, fmt.Errorf("unexpected type at field %s, expected map[string]interface{} or []interface{}, got %T", strings.Join(fields[:i+1], "."), current)
		}
	}

	return current, true, nil
}

func CreateSubscriptionResolver(isSingle bool) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		source := p.Source

		if err, ok := source.(error); ok {
			return nil, err
		}

		return source, nil
	}
}
