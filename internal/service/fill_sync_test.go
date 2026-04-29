package service

import (
	"reflect"
	"testing"
)

func TestMaskSensitiveData(t *testing.T) {
	input := map[string]any{
		"orderId":     "12345",
		"apiKey":      "my-secret-api-key",
		"APISecret":   "super-secret",
		"signature":   "abcdef123456",
		"clientToken": "tok_123",
		"nested": map[string]any{
			"normal": "value",
			"key":    "nested-key",
		},
		"list": []any{
			map[string]any{
				"itemKey": "val1",
			},
			"normal_string",
		},
	}

	expected := map[string]any{
		"orderId":     "12345",
		"apiKey":      "***",
		"APISecret":   "***",
		"signature":   "***",
		"clientToken": "***",
		"nested": map[string]any{
			"normal": "value",
			"key":    "***",
		},
		"list": []any{
			map[string]any{
				"itemKey": "***",
			},
			"normal_string",
		},
	}

	result := maskSensitiveData(input)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("maskSensitiveData() = %v, want %v", result, expected)
	}
}
