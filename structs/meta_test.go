package structs

import (
	"strings"
	"testing"
)

func TestAddWellKnownMetaProperties(t *testing.T) {
	t.Run("add single property", func(t *testing.T) {
		original := GetWellKnownMetaProperties()
		defer func() {
			RemoveWellKnownMetaProperties(original...)
			RemoveWellKnownMetaProperties("test-prop")
		}()

		AddWellKnownMetaProperties("test-prop")

		if !HasWellKnownMetaProperty("test-prop") {
			t.Error("Expected 'test-prop' to be a well-known meta property")
		}
	})

	t.Run("add multiple properties", func(t *testing.T) {
		original := GetWellKnownMetaProperties()
		defer func() {
			RemoveWellKnownMetaProperties(original...)
			RemoveWellKnownMetaProperties("prop1", "prop2", "prop3")
		}()

		AddWellKnownMetaProperties("prop1", "prop2", "prop3")

		if !HasWellKnownMetaProperty("prop1") {
			t.Error("Expected 'prop1' to be a well-known meta property")
		}
		if !HasWellKnownMetaProperty("prop2") {
			t.Error("Expected 'prop2' to be a well-known meta property")
		}
		if !HasWellKnownMetaProperty("prop3") {
			t.Error("Expected 'prop3' to be a well-known meta property")
		}
	})

	t.Run("add duplicate property is ignored", func(t *testing.T) {
		original := GetWellKnownMetaProperties()
		defer func() {
			RemoveWellKnownMetaProperties(original...)
			RemoveWellKnownMetaProperties("test-prop")
		}()

		AddWellKnownMetaProperties("test-prop")
		propsAfterFirstAdd := GetWellKnownMetaProperties()

		AddWellKnownMetaProperties("test-prop")
		propsAfterSecondAdd := GetWellKnownMetaProperties()

		if len(propsAfterFirstAdd) != len(propsAfterSecondAdd) {
			t.Error("Expected duplicate property to be ignored")
		}
	})
}

func TestHasWellKnownMetaProperty(t *testing.T) {
	t.Run("non-existent property returns false", func(t *testing.T) {
		if HasWellKnownMetaProperty("non-existent") {
			t.Error("Expected false for non-existent property")
		}
	})

	t.Run("existing property returns true", func(t *testing.T) {
		original := GetWellKnownMetaProperties()
		defer func() {
			RemoveWellKnownMetaProperties(original...)
			RemoveWellKnownMetaProperties("test-prop")
		}()

		AddWellKnownMetaProperties("test-prop")

		if !HasWellKnownMetaProperty("test-prop") {
			t.Error("Expected true for existing property")
		}
	})
}

func TestRemoveWellKnownMetaProperties(t *testing.T) {
	t.Run("remove single property", func(t *testing.T) {
		original := GetWellKnownMetaProperties()
		defer func() {
			RemoveWellKnownMetaProperties(original...)
		}()

		AddWellKnownMetaProperties("test-prop")

		RemoveWellKnownMetaProperties("test-prop")

		if HasWellKnownMetaProperty("test-prop") {
			t.Error("Expected 'test-prop' to be removed")
		}
	})

	t.Run("remove multiple properties", func(t *testing.T) {
		original := GetWellKnownMetaProperties()
		defer func() {
			RemoveWellKnownMetaProperties(original...)
		}()

		AddWellKnownMetaProperties("prop1", "prop2", "prop3")

		RemoveWellKnownMetaProperties("prop1", "prop2")

		if HasWellKnownMetaProperty("prop1") {
			t.Error("Expected 'prop1' to be removed")
		}
		if HasWellKnownMetaProperty("prop2") {
			t.Error("Expected 'prop2' to be removed")
		}
		if !HasWellKnownMetaProperty("prop3") {
			t.Error("Expected 'prop3' to still exist")
		}
	})

	t.Run("remove non-existent property is no-op", func(t *testing.T) {
		original := GetWellKnownMetaProperties()
		defer func() {
			RemoveWellKnownMetaProperties(original...)
		}()

		RemoveWellKnownMetaProperties("non-existent")
	})
}

func TestGetWellKnownMetaProperties(t *testing.T) {
	t.Run("returns copy not reference", func(t *testing.T) {
		original := GetWellKnownMetaProperties()
		defer func() {
			RemoveWellKnownMetaProperties(original...)
		}()

		props1 := GetWellKnownMetaProperties()
		AddWellKnownMetaProperties("test-prop")
		props2 := GetWellKnownMetaProperties()

		if len(props1) == len(props2) {
			t.Error("Expected returned slice to be a copy, not a reference")
		}
	})
}

type customStringer struct{}

func (c customStringer) String() string {
	return "custom-value"
}

func TestMetaString(t *testing.T) {
	t.Run("nil meta returns empty string", func(t *testing.T) {
		var meta Meta
		metaStr := meta.String()
		if metaStr != "" {
			t.Errorf("Expected empty string, got %s", metaStr)
		}
	})

	t.Run("empty meta returns empty string", func(t *testing.T) {
		meta := Meta{}
		metaStr := meta.String()
		if metaStr != "" {
			t.Errorf("Expected empty string, got %s", metaStr)
		}
	})

	t.Run("meta with no well-known properties returns empty string", func(t *testing.T) {
		original := GetWellKnownMetaProperties()
		defer func() {
			RemoveWellKnownMetaProperties(original...)
		}()

		meta := Meta{"unknown": "value"}
		metaStr := meta.String()
		if metaStr != "" {
			t.Errorf("Expected empty string, got %s", metaStr)
		}
	})

	t.Run("meta with well-known property", func(t *testing.T) {
		original := GetWellKnownMetaProperties()
		defer func() {
			RemoveWellKnownMetaProperties(original...)
		}()

		AddWellKnownMetaProperties("addr")
		meta := Meta{"addr": "192.168.1.1"}
		metaStr := meta.String()
		if !strings.Contains(metaStr, "192.168.1.1") {
			t.Errorf("Expected metaStr to contain '192.168.1.1', got %s", metaStr)
		}
	})

	t.Run("meta with multiple well-known properties", func(t *testing.T) {
		original := GetWellKnownMetaProperties()
		defer func() {
			RemoveWellKnownMetaProperties(original...)
		}()

		AddWellKnownMetaProperties("addr", "method", "path")
		meta := Meta{
			"addr":   "192.168.1.1",
			"method": "GET",
			"path":   "/api/test",
		}
		metaStr := meta.String()
		if !strings.Contains(metaStr, "192.168.1.1") {
			t.Errorf("Expected metaStr to contain '192.168.1.1', got %s", metaStr)
		}
		if !strings.Contains(metaStr, "GET") {
			t.Errorf("Expected metaStr to contain 'GET', got %s", metaStr)
		}
		if !strings.Contains(metaStr, "/api/test") {
			t.Errorf("Expected metaStr to contain '/api/test', got %s", metaStr)
		}
	})

	t.Run("meta with non-string value is skipped", func(t *testing.T) {
		original := GetWellKnownMetaProperties()
		defer func() {
			RemoveWellKnownMetaProperties(original...)
		}()

		AddWellKnownMetaProperties("addr")
		meta := Meta{"addr": 12345}
		metaStr := meta.String()
		if metaStr != "" {
			t.Errorf("Expected empty string for non-string value, got %s", metaStr)
		}
	})

	t.Run("meta with Stringer value", func(t *testing.T) {
		original := GetWellKnownMetaProperties()
		defer func() {
			RemoveWellKnownMetaProperties(original...)
		}()

		AddWellKnownMetaProperties("custom")
		meta := Meta{"custom": customStringer{}}
		metaStr := meta.String()
		if !strings.Contains(metaStr, "custom-value") {
			t.Errorf("Expected metaStr to contain 'custom-value', got %s", metaStr)
		}
	})

	t.Run("empty well-known properties returns empty string", func(t *testing.T) {
		original := GetWellKnownMetaProperties()
		defer func() {
			RemoveWellKnownMetaProperties(original...)
		}()

		meta := Meta{"any": "value"}
		metaStr := meta.String()
		if metaStr != "" {
			t.Errorf("Expected empty string when no well-known properties, got %s", metaStr)
		}
	})
}
