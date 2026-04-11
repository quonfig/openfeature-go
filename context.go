package openfeaturego

import (
	"strings"

	quonfig "github.com/quonfig/sdk-go"
)

// MapContext converts an OpenFeature flat FlattenedContext to a Quonfig ContextSet.
//
// Mapping rules:
//   - "user.email" -> namespace "user", key "email"
//   - "country"    -> namespace "",     key "country"  (no dot -> empty-string namespace)
//   - "user.ip.address" -> namespace "user", key "ip.address" (split on first dot only)
//   - "targetingKey" -> resolved via targetingKeyMapping (default "user.id")
func MapContext(flatCtx map[string]any, targetingKeyMapping string) *quonfig.ContextSet {
	if len(flatCtx) == 0 {
		return nil
	}

	namespaces := make(map[string]map[string]interface{})

	for k, v := range flatCtx {
		if v == nil {
			continue
		}
		if k == "targetingKey" {
			ns, prop := splitFirst(targetingKeyMapping, ".")
			addToNS(namespaces, ns, prop, v)
			continue
		}
		ns, prop := splitFirst(k, ".")
		addToNS(namespaces, ns, prop, v)
	}

	if len(namespaces) == 0 {
		return nil
	}

	ctxSet := quonfig.NewContextSet()
	for ns, values := range namespaces {
		ctxSet.WithNamedContextValues(ns, values)
	}
	return ctxSet
}

// splitFirst splits s on the first occurrence of sep.
// If sep is not found, returns ("", s) so the whole key ends up in the empty-string namespace.
func splitFirst(s, sep string) (string, string) {
	idx := strings.Index(s, sep)
	if idx == -1 {
		return "", s
	}
	return s[:idx], s[idx+1:]
}

func addToNS(namespaces map[string]map[string]interface{}, ns, prop string, v interface{}) {
	if _, ok := namespaces[ns]; !ok {
		namespaces[ns] = make(map[string]interface{})
	}
	namespaces[ns][prop] = v
}
