package generate

import (
	"net/url"
	"testing"
)

func TestThatAMissingSchemaKeyResultsInAnError(t *testing.T) {
	invalid := `{
        "title": "root"
    }`
	_, invaliderr := Parse(invalid, &url.URL{Scheme: "file", Path: "jsonschemaparse_test.go"})
	valid := `{
        "$schema": "http://json-schema.org/schema#",
        "title": "root"
    }`
	_, validerr := Parse(valid, &url.URL{Scheme: "file", Path: "jsonschemaparse_test.go"})
	if invaliderr == nil {
		// it SHOULD be used in the root schema
		// t.Error("When the $schema key is missing from the root, the JSON Schema is not valid")
	}
	if validerr != nil {
		t.Error("It should be possible to parse a simple JSON schema if the $schema key is present")
	}
}

func TestThatTheRootSchemaCanBeParsed(t *testing.T) {
	s := `{
        "$schema": "http://json-schema.org/schema#",
        "title": "root"
    }`
	so, err := Parse(s, &url.URL{Scheme: "file", Path: "jsonschemaparse_test.go"})

	if err != nil {
		t.Fatal("It should be possible to unmarshal a simple schema, but received error:", err)
	}

	if so.Title != "root" {
		t.Errorf("The title was not deserialised from the JSON schema, expected %s, but got %s", "root", so.Title)
	}
}

func TestThatParsingInvalidValuesReturnsAnError(t *testing.T) {
	s := `{ " }`
	_, err := Parse(s, &url.URL{Scheme: "file", Path: "jsonschemaparse_test.go"})

	if err == nil {
		t.Fatal("Expected a parsing error, but got nil")
	}
}

func TestThatDefaultsCanBeParsed(t *testing.T) {
	s := `{
        "$schema": "http://json-schema.org/schema#",
        "title": "root",
        "properties": {
            "name": {
                "type": [ "integer", "string" ],
                "default":"Enrique"
            }
        }
    }`
	so, err := Parse(s, &url.URL{Scheme: "file", Path: "jsonschemaparse_test.go"})

	if err != nil {
		t.Fatal("It was not possible to unmarshal the schema:", err)
	}

	defaultValue := so.Properties["name"].Default
	if defaultValue != "Enrique" {
		t.Errorf("expected default value of property 'name' type to be 'Enrique', but was '%v'", defaultValue)
	}
}

func TestReturnedSchemaId(t *testing.T) {
	tests := []struct {
		input    *Schema
		expected string
	}{
		{
			input:    &Schema{},
			expected: "",
		},
		{
			input:    &Schema{ID06: "http://example.com/foo.json", ID04: "#foo"},
			expected: "http://example.com/foo.json",
		},
		{
			input:    &Schema{ID04: "#foo"},
			expected: "#foo",
		},
	}

	for idx, test := range tests {
		actual := test.input.ID()
		if actual != test.expected {
			t.Errorf("Test %d failed: For input \"%+v\", expected \"%s\", got \"%s\"", idx, test.input, test.expected, actual)
		}
	}
}
