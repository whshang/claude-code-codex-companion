package json

import (
	"testing"
)

func TestSafeMarshal(t *testing.T) {
	testCases := []struct {
		name     string
		input    interface{}
		wantErr  bool
	}{
		{
			name:    "simple string",
			input:   "hello",
			wantErr: false,
		},
		{
			name:    "map",
			input:   map[string]interface{}{"key": "value"},
			wantErr: false,
		},
		{
			name:    "nil",
			input:   nil,
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := SafeMarshal(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("SafeMarshal() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !tc.wantErr && len(data) == 0 {
				t.Errorf("SafeMarshal() returned empty data")
			}
		})
	}
}

func TestSafeUnmarshal(t *testing.T) {
	testData := []byte(`{"name": "test", "value": 123}`)
	
	var result map[string]interface{}
	err := SafeUnmarshal(testData, &result)
	if err != nil {
		t.Errorf("SafeUnmarshal() error = %v", err)
	}
	
	if result["name"] != "test" {
		t.Errorf("Expected name 'test', got %v", result["name"])
	}
}

func TestExtractField(t *testing.T) {
	testData := []byte(`{"name": "test", "count": 42, "active": true}`)
	
	// Test string field
	name, err := ExtractField[string](testData, "name")
	if err != nil {
		t.Errorf("ExtractField[string]() error = %v", err)
	}
	if name != "test" {
		t.Errorf("Expected 'test', got '%s'", name)
	}
	
	// Test number field (JSON numbers are float64)
	count, err := ExtractField[float64](testData, "count")
	if err != nil {
		t.Errorf("ExtractField[float64]() error = %v", err)
	}
	if count != 42.0 {
		t.Errorf("Expected 42.0, got %f", count)
	}
	
	// Test boolean field
	active, err := ExtractField[bool](testData, "active")
	if err != nil {
		t.Errorf("ExtractField[bool]() error = %v", err)
	}
	if !active {
		t.Errorf("Expected true, got %v", active)
	}
	
	// Test non-existent field
	missing, err := ExtractField[string](testData, "missing")
	if err != nil {
		t.Errorf("ExtractField() for missing field error = %v", err)
	}
	if missing != "" {
		t.Errorf("Expected empty string for missing field, got '%s'", missing)
	}
}

func TestIsValidJSON(t *testing.T) {
	testCases := []struct {
		name  string
		data  []byte
		valid bool
	}{
		{
			name:  "valid object",
			data:  []byte(`{"key": "value"}`),
			valid: true,
		},
		{
			name:  "valid array",
			data:  []byte(`[1, 2, 3]`),
			valid: true,
		},
		{
			name:  "invalid JSON",
			data:  []byte(`{"key": value}`),
			valid: false,
		},
		{
			name:  "empty",
			data:  []byte(``),
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsValidJSON(tc.data)
			if result != tc.valid {
				t.Errorf("IsValidJSON() = %v, want %v", result, tc.valid)
			}
		})
	}
}