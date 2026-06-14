package kubernetes

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// obj is shorthand for an unstructured Kubernetes object's content map.
type obj = map[string]any

func str(o obj, fields ...string) string {
	v, _, _ := unstructured.NestedString(o, fields...)
	return v
}

func i64(o obj, fields ...string) int64 {
	v, _, _ := unstructured.NestedInt64(o, fields...)
	return v
}

func boolField(o obj, fields ...string) bool {
	v, _, _ := unstructured.NestedBool(o, fields...)
	return v
}

func slice(o obj, fields ...string) []any {
	v, _, _ := unstructured.NestedSlice(o, fields...)
	return v
}

func mapField(o obj, fields ...string) map[string]any {
	v, _, _ := unstructured.NestedMap(o, fields...)
	return v
}

func createdAt(o obj) string { return str(o, "metadata", "creationTimestamp") }

// commonRow seeds the fields every kind shares; kind mappers add the rest.
func commonRow(o obj) Row {
	created := createdAt(o)
	return Row{
		"name":      str(o, "metadata", "name"),
		"namespace": str(o, "metadata", "namespace"),
		"uid":       str(o, "metadata", "uid"),
		"age":       created,
		"createdAt": created,
	}
}

// ref builds the ResourceIdentity identity for a tree node from an object.
func refName(o obj) string { return str(o, "metadata", "name") }
func refNS(o obj) string   { return str(o, "metadata", "namespace") }

func joinStrings(items []string) string { return strings.Join(items, ", ") }

// scalar renders a nested field that may be an int or a string (e.g. an
// IntOrString like minAvailable) as a display string.
func scalar(o obj, fields ...string) string {
	v, found, _ := unstructured.NestedFieldNoCopy(o, fields...)
	if !found || v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
