# generate

Generates Go (golang) Structs from JSON schema.

# Requirements

* Go 1.22+

# Usage

Install

```console
$ go install github.com/Graff913/generate-go-json-schema/cmd/schema-generate
```

or

Build

```console
$ make
```

Run

```console
$ schema-generate exampleschema.json
```

# Example

This schema

```json
{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "title": "Example",
  "id": "http://example.com/exampleschema.json",
  "type": "object",
  "description": "An example JSON Schema",
  "properties": {
    "name": {
      "type": "string"
    },
    "address": {
      "$ref": "#/$defs/address"
    },
    "status": {
      "$ref": "#/$defs/status"
    }
  },
  "$defs": {
    "address": {
      "id": "address",
      "type": "object",
      "description": "Address",
      "properties": {
        "street": {
          "type": "string",
          "description": "Address 1",
          "maxLength": 40
        },
        "houseNumber": {
          "type": "integer",
          "description": "House Number"
        }
      }
    },
    "status": {
      "type": "object",
      "properties": {
        "favouriteCat": {
          "enum": [
            "A",
            "B",
            "C"
          ],
          "type": "string",
          "description": "The favourite cat.",
          "maxLength": 1
        }
      }
    }
  }
}
```

generates

```go
package main

// Address Address
type Address struct {

	// House Number
	HouseNumber *int `json:"houseNumber,omitempty"`

	// Address 1
	Street *string `json:"street,omitempty"`
}

// Example An example JSON Schema
type Example struct {
	Address *Address `json:"address,omitempty"`
	Name *string `json:"name,omitempty"`
	Status *Status `json:"status,omitempty"`
}

// FavouriteCat The favourite cat.
type FavouriteCat string

const (
	FavouriteCatA FavouriteCat = "A"
	FavouriteCatB FavouriteCat = "B"
	FavouriteCatC FavouriteCat = "C"
)

// Status 
type Status struct {

	// The favourite cat.
	FavouriteCat FavouriteCat `json:"favouriteCat,omitempty"`
}
```
