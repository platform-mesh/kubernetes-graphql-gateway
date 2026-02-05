package resolver

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/pkg/errors"
	"github.com/platform-mesh/golang-commons/sentry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrFailedToCastEventObjectToUnstructured = fmt.Errorf("failed to cast event object to unstructured")
)

// Event type constants used in subscription envelopes
const (
	EventTypeAdded    = "ADDED"
	EventTypeModified = "MODIFIED"
	EventTypeDeleted  = "DELETED"
)

// SubscriptionEnvelope represents the envelope for a subscription update
type SubscriptionEnvelope struct {
	Type   string `json:"type"`
	Object any    `json:"object"`
}

// SubscriptionObject represents an object with only minimal metadata
type SubscriptionObject struct {
	Metadata SubscriptionMetadata `json:"metadata"`
}

// SubscriptionMetadata represents minimal metadata for an object
type SubscriptionMetadata struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace,omitempty"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

func (r *Service) SubscribeItem(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		_, span := otel.Tracer("").Start(p.Context, "SubscribeItem", trace.WithAttributes(attribute.String("kind", gvk.Kind)))
		defer span.End()
		resultChannel := make(chan any)
		go r.runWatch(p, gvk, resultChannel, true, scope)

		return resultChannel, nil
	}
}

func (r *Service) SubscribeItems(gvk schema.GroupVersionKind, scope v1.ResourceScope) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		_, span := otel.Tracer("").Start(p.Context, "SubscribeItems", trace.WithAttributes(attribute.String("kind", gvk.Kind)))
		defer span.End()
		resultChannel := make(chan any)
		go r.runWatch(p, gvk, resultChannel, false, scope)

		return resultChannel, nil
	}
}

func (r *Service) runWatch(
	p graphql.ResolveParams,
	gvk schema.GroupVersionKind,
	resultChannel chan any,
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

	// optional resourceVersion to continue subscription from
	resourceVersion, err := getStringArg(p.Args, ResourceVersionArg, false)
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to get resourceVersion argument")
		resultChannel <- errors.Wrap(err, "failed to get resourceVersion argument")
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

	// If no resourceVersion provided, perform an initial LIST to obtain current items and resourceVersion,
	// If a resourceVersion is provided, start WATCH from that resourceVersion without initial listing.

	var watchOpts []client.ListOption
	watchOpts = append(watchOpts, opts...)

	// Track last-seen objects for change detection on MODIFIED
	previousObjects := make(map[string]*unstructured.Unstructured)

	if resourceVersion == "" {
		// Initial LIST without a resourceVersion to get current items and resourceVersion
		if err := r.runtimeClient.List(ctx, list, opts...); err != nil {
			r.log.Error().Err(err).Str("gvk", gvk.String()).Msg("Failed to list resources for initial watch state")
			sentry.CaptureError(err, sentry.Tags{"namespace": namespace}, sentry.Extras{"gvk": gvk.String()})
			resultChannel <- errors.Wrap(err, "failed to list resources for initial watch state")
			return
		}

		for i := range list.Items {
			item := list.Items[i]
			key := item.GetNamespace() + "/" + item.GetName()
			previousObjects[key] = item.DeepCopy()

			envelope := SubscriptionEnvelope{
				Type:   EventTypeAdded,
				Object: item.Object,
			}
			select {
			case <-ctx.Done():
				return
			case resultChannel <- envelope:
			}
		}

		// Start WATCH from the LIST's resourceVersion
		rv := list.GetResourceVersion()
		if rv != "" {
			watchOpts = append(watchOpts, &client.ListOptions{Raw: &metav1.ListOptions{ResourceVersion: rv}})
		}
	} else {
		// Start WATCH from provided resourceVersion
		watchOpts = append(watchOpts, &client.ListOptions{Raw: &metav1.ListOptions{ResourceVersion: resourceVersion}})
	}

	watcher, err := r.runtimeClient.Watch(ctx, list, watchOpts...)
	if err != nil {
		r.log.Error().Err(err).Str("gvk", gvk.String()).Msg("Failed to start watch")

		sentry.CaptureError(err, sentry.Tags{"namespace": namespace}, sentry.Extras{"gvk": gvk.String()})

		resultChannel <- errors.Wrap(err, "failed to start watch")
		return
	}
	defer watcher.Stop()

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
			var eventType string
			switch event.Type {
			case watch.Added:
				previousObjects[key] = obj.DeepCopy()
				sendUpdate = true
				eventType = EventTypeAdded
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
				if sendUpdate {
					eventType = EventTypeModified
				}
			case watch.Deleted:
				delete(previousObjects, key)
				sendUpdate = true
				eventType = EventTypeDeleted
			}

			if sendUpdate {
				var payload any = obj.Object

				// Ensure payload is a non-nil map; if not, provide minimal metadata so
				// clients can reconcile caches
				if m, ok := payload.(map[string]any); !ok || m == nil {
					payload = SubscriptionObject{
						Metadata: SubscriptionMetadata{
							Name:            obj.GetName(),
							Namespace:       obj.GetNamespace(),
							ResourceVersion: obj.GetResourceVersion(),
						},
					}
				}

				envelope := SubscriptionEnvelope{
					Type:   eventType,
					Object: payload,
				}

				select {
				case <-ctx.Done():
					return
				case resultChannel <- envelope:
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
		if fieldAST.SelectionSet == nil {
			continue
		}
		for _, sel := range fieldAST.SelectionSet.Selections {
			if f, ok := sel.(*ast.Field); ok {
				if f.Name.Value == "object" {
					fields = append(fields, parseSelectionSet(f.SelectionSet, "")...)
				}
			}
		}
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
func getFieldValue(obj *unstructured.Unstructured, fieldPath string) (any, bool, error) {
	fields := strings.Split(fieldPath, ".")
	var current any = obj.Object

	for i, field := range fields {
		switch v := current.(type) {
		case map[string]any:
			value, found, err := unstructured.NestedFieldNoCopy(v, field)
			if err != nil {
				return nil, false, fmt.Errorf("error accessing field %s: %v", strings.Join(fields[:i+1], "."), err)
			}
			if !found {
				return nil, false, nil
			}
			current = value
		case []any:
			// in case of slice, we return it, and that slice will be compared later using deep equal
			return current, true, nil
		default:
			return nil, false, fmt.Errorf("unexpected type at field %s, expected map[string]interface{} or []interface{}, got %T", strings.Join(fields[:i+1], "."), current)
		}
	}

	return current, true, nil
}

func CreateSubscriptionResolver(isSingle bool) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (any, error) {
		source := p.Source

		if err, ok := source.(error); ok {
			return nil, err
		}

		return source, nil
	}
}
