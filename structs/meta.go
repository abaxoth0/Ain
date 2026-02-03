package structs

import (
	"fmt"
	"slices"
)

type Meta map[string]any

func (m Meta) String() string {
	if m == nil || len(wellKnownMetaProperties) == 0 {
		return ""
	}
	s := ""
	for _, prop := range wellKnownMetaProperties {
		switch v := m[prop].(type) {
		case string:
			s += v + " "
		case Meta:
			panic(fmt.Sprintf("Invalid data type of Meta[\"%s\"]: Meta", prop))
		}
		if v, ok := m[prop].(fmt.Stringer); ok {
			s += v.String() + " "
		}
	}
	if s == "" {
		return ""
	}
	return "(" + s + ")"
}

var wellKnownMetaProperties []string

// Adds a new well-known meta property(-s), but only if it doesn't already exist.
func AddWellKnownMetaProperties(props ...string) {
	for _, prop := range props {
		if HasWellKnownMetaProperty(prop) {
			continue
		}
		wellKnownMetaProperties = append(wellKnownMetaProperties, prop)
	}
}

// Reports if specified property exists is well-known meta properties.
func HasWellKnownMetaProperty(prop string) bool {
	return slices.Contains(wellKnownMetaProperties, prop)
}

// Remove specified well-known meta property(-s).
func RemoveWellKnownMetaProperties(props ...string) {
	wellKnownMetaProperties = slices.DeleteFunc(wellKnownMetaProperties, func(prop string) bool {
		return slices.Contains(props, prop)
	})
}

// Returns a copy of a slice with all well-known meta properties
func GetWellKnownMetaProperties() []string {
	r := make([]string, len(wellKnownMetaProperties))
	copy(r, wellKnownMetaProperties)
	return r
}
