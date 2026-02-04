package structs

import (
	"fmt"
	"slices"
)

// Represents metadata as a map of string keys to any values.
type Meta map[string]any

// Returns a formatted string representation of well-known metadata properties.
// Only properties registered as well-known will be included in the output.
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

// stores globally registered well-known metadata property names.
var wellKnownMetaProperties []string

// Adds new well-known meta properties, but only if they don't already exist.
func AddWellKnownMetaProperties(props ...string) {
	for _, prop := range props {
		if HasWellKnownMetaProperty(prop) {
			continue
		}
		wellKnownMetaProperties = append(wellKnownMetaProperties, prop)
	}
}

// Reports if the specified property exists in well-known meta properties.
func HasWellKnownMetaProperty(prop string) bool {
	return slices.Contains(wellKnownMetaProperties, prop)
}

// Removes specified well-known meta properties.
func RemoveWellKnownMetaProperties(props ...string) {
	wellKnownMetaProperties = slices.DeleteFunc(wellKnownMetaProperties, func(prop string) bool {
		return slices.Contains(props, prop)
	})
}

// Returns a copy of a slice with all well-known meta properties.
func GetWellKnownMetaProperties() []string {
	r := make([]string, len(wellKnownMetaProperties))
	copy(r, wellKnownMetaProperties)
	return r
}
