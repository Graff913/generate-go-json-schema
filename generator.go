package generate

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Generator will produce structs from the JSON schema.
type Generator struct {
	schemas  []*Schema
	resolver *RefResolver
	Structs  map[string]Struct
	Aliases  map[string]Field
	// cache for reference types; k=url v=type
	refs      map[string]string
	anonCount int
}

// New creates an instance of a generator which will produce structs.
func New(schemas ...*Schema) *Generator {
	return &Generator{
		schemas:  schemas,
		resolver: NewRefResolver(schemas),
		Structs:  make(map[string]Struct),
		Aliases:  make(map[string]Field),
		refs:     make(map[string]string),
	}
}

// CreateTypes creates types from the JSON schemas, keyed by the golang name.
func (g *Generator) CreateTypes(rootPath, pkg string, bson bool) (err error) {
	if err := g.resolver.Init(); err != nil {
		return err
	}

	// extract the types
	for _, schema := range g.schemas {
		name := g.getSchemaName("", schema)
		_, err := g.processSchema(rootPath, pkg, name, bson, false, schema) // rootType
		if err != nil {
			return err
		}
		// ugh: if it was anything but a struct the type will not be the name...
		//if rootType != "*"+name {
		//	a := Field{
		//		Name:        name,
		//		JSONName:    "",
		//		Type:        rootType,
		//		Required:    false,
		//		Description: schema.Description,
		//	}
		//	g.Aliases[a.Name] = a
		//}
	}
	return
}

// process a block of $defs
func (g *Generator) processDefinitions(rootPath, pkg string, schema *Schema) error {
	for key, subSchema := range schema.Definitions {
		if _, err := g.processSchema(rootPath, pkg, getGolangName(key), false, false, subSchema); err != nil {
			return err
		}
	}
	return nil
}

// process a reference string
func (g *Generator) processReference(rootPath, pkg string, schema *Schema, requires bool) (string, error) {
	schemaPath := g.resolver.GetPath(schema)
	if schema.Reference == "" {
		return "", errors.New("processReference empty reference: " + schemaPath)
	}
	refSchema, err := g.resolver.GetSchemaByReference(rootPath, schema)
	if err != nil {
		return "", errors.New("processReference: reference \"" + schema.Reference + "\" not found at \"" + schemaPath + "\"")
	}
	if refSchema.GeneratedType == "" {
		// reference is not resolved yet. Do that now.
		refSchemaName := g.getSchemaName("", refSchema)
		typeName, err := g.processSchema(rootPath, pkg, refSchemaName, false, requires, refSchema)
		if err != nil {
			return "", err
		}
		return typeName, nil
	}
	return refSchema.GeneratedType, nil
}

// returns the type refered to by schema after resolving all dependencies
func (g *Generator) processSchema(rootPath, pkg string, schemaName string, bson, requires bool, schema *Schema) (typ string, err error) {
	if len(schema.Definitions) > 0 {
		err := g.processDefinitions(rootPath, pkg, schema)
		if err != nil {
			return "", err
		}
	}
	schema.FixMissingTypeValue()
	// if we have multiple schema types, the golang type will be interface{}
	typ = "interface{}"
	types, isMultiType, _ := schema.MultiType()
	if len(types) > 0 {
		for _, schemaType := range types {
			name := schemaName
			if isMultiType {
				name = name + "_" + schemaType
			}
			switch schemaType {
			case "object":
				rv, err := g.processObject(rootPath, pkg, name, bson, schema)
				if err != nil {
					return "", err
				}
				if !isMultiType {
					return rv, nil
				}
			case "array":
				rv, err := g.processArray(rootPath, pkg, name, schema)
				if err != nil {
					return "", err
				}
				if !isMultiType {
					return rv, nil
				}
			default:
				rv, err := getPrimitiveTypeName(schemaType, "")
				if err != nil {
					return "", err
				}
				if !isMultiType {
					return rv, nil
				}
			}
		}
	} else {
		if schema.Reference != "" {
			return g.processReference(rootPath, pkg, schema, requires)
		}
		if len(schema.OneOf) > 0 {
			return g.processInterface(rootPath, pkg, schemaName, requires, schema)
		}
		if len(schema.EnumValue) > 0 {
			return g.processEnum(schemaName, schema)
		}
	}
	return // return interface{}
}

// name: name of this array, usually the js key
// schema: items element
func (g *Generator) processArray(rootPath, pkg string, name string, schema *Schema) (typeStr string, err error) {
	if schema.Items != nil {
		// subType: fallback name in case this array contains inline object without a title
		subName := g.getSchemaName(name+"Items", schema.Items)
		subTyp, err := g.processSchema(rootPath, pkg, subName, false, true, schema.Items)
		if err != nil {
			return "", err
		}
		finalType, err := getPrimitiveTypeName("array", subTyp)
		if err != nil {
			return "", err
		}
		// only alias root arrays
		if schema.Parent == nil {
			array := Field{
				Name:        name,
				JSONName:    "",
				Type:        finalType,
				Required:    contains(schema.Required, name),
				Description: schema.Description,
			}
			g.Aliases[array.Name] = array
		}
		return finalType, nil
	}
	return "[]interface{}", nil
}

// name: name of the struct (calculated by caller)
// schema: detail incl properties & child objects
// returns: generated type
func (g *Generator) processObject(rootPath, pkg string, name string, bson bool, schema *Schema) (typ string, err error) {
	strct := Struct{
		ID:          schema.ID(),
		Name:        name,
		Description: schema.Description,
		Fields:      make(map[string]Field, len(schema.Properties)),
	}
	// cache the object name in case any sub-schemas recursively reference it
	schema.GeneratedType = name
	// regular properties
	if bson && schema.Root {
		f := Field{
			Name:     "ObjectId",
			JSONName: "_id",
			Type:     "primitive.ObjectID",
			Required: false,
		}
		strct.Fields[f.Name] = f
	}
	for propKey, prop := range schema.Properties {
		fieldName := getGolangName(propKey)
		// calculate sub-schema name here, may not actually be used depending on type of schema!
		subSchemaName := g.getSchemaName(fieldName, prop)
		fieldType, err := g.processSchema(rootPath, pkg, subSchemaName, false, contains(schema.Required, propKey), prop)
		if err != nil {
			return "", err
		}
		f := Field{
			Name:        fieldName,
			JSONName:    propKey,
			Type:        fieldType,
			Required:    contains(schema.Required, propKey),
			Description: prop.Description,
		}
		if prop.Deprecated {
			f.Description = "@deprecated: " + prop.Description
		}
		if f.Required {
			strct.GenerateCode = true
		}
		strct.Fields[f.Name] = f

	}
	// additionalProperties with typed sub-schema
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.AdditionalPropertiesBool == nil {
		ap := (*Schema)(schema.AdditionalProperties)
		apName := g.getSchemaName("", ap)
		subTyp, err := g.processSchema(rootPath, pkg, apName, false, true, ap)
		if err != nil {
			return "", err
		}
		mapTyp := "map[string]" + subTyp
		// If this object is inline property for another object, and only contains additional properties, we can
		// collapse the structure down to a map.
		//
		// If this object is a definition and only contains additional properties, we can't do that or we end up with
		// no struct
		isDefinitionObject := strings.HasPrefix(schema.PathElement, "$defs")
		if len(schema.Properties) == 0 && !isDefinitionObject {
			// since there are no regular properties, we don't need to emit a struct for this object - return the
			// additionalProperties map type.
			return mapTyp, nil
		}
		// this struct will have both regular and additional properties
		f := Field{
			Name:        "AdditionalProperties",
			JSONName:    "-",
			Type:        mapTyp,
			Required:    false,
			Description: "",
		}
		strct.Fields[f.Name] = f
		// setting this will cause marshal code to be emitted in Output()
		strct.GenerateCode = true
		strct.AdditionalType = subTyp
	}
	// additionalProperties as either true (everything) or false (nothing)
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.AdditionalPropertiesBool != nil {
		if *schema.AdditionalProperties.AdditionalPropertiesBool {
			// everything is valid additional
			subTyp := "map[string]interface{}"
			f := Field{
				Name:        "AdditionalProperties",
				JSONName:    "-",
				Type:        subTyp,
				Required:    false,
				Description: "",
			}
			strct.Fields[f.Name] = f
			// setting this will cause marshal code to be emitted in Output()
			strct.GenerateCode = true
			strct.AdditionalType = "interface{}"
		} else {
			// nothing
			strct.GenerateCode = true
			strct.AdditionalType = "false"
		}
	}
	if len(strct.Fields) == 0 {
		return "map[string]interface{}", nil
	}

	g.Structs[strct.Name] = strct

	// objects are always a pointer
	return getPrimitiveTypeName("object", name)
}

func (g *Generator) processInterface(rootPath, pkg string, name string, requires bool, schema *Schema) (typ string, err error) {
	name = name + "Interface"
	strct := Struct{
		ID:          schema.ID(),
		Name:        name,
		Description: schema.Description,
		Func: Func{
			Name:      "Is" + toTitle(pkg) + name,
			NameTypes: nil,
		},
	}

	var refer *string
	for _, oneOf := range schema.OneOf {
		if oneOf.Reference != "" {
			refer = &oneOf.Reference
			split := strings.Split(oneOf.Reference, "/")
			strct.Func.NameTypes = append(strct.Func.NameTypes, split[len(split)-1])
		}
	}

	if len(strct.Func.NameTypes) == 1 {
		schema.Reference = *refer
		return g.processReference(rootPath, pkg, schema, requires)
	}

	g.Structs[strct.Name] = strct

	return name, nil
}

func (g *Generator) processEnum(name string, schema *Schema) (typ string, err error) {
	strct := Struct{
		ID:          schema.ID(),
		Name:        name,
		Description: schema.Description,
	}

	for _, val := range schema.EnumValue {
		customName := name

		switch v := val.(type) {
		case string:
			f := func(c rune) bool {
				return !unicode.IsLetter(c) && !unicode.IsNumber(c)
			}
			n := strings.FieldsFunc(v, f)
			for i := 0; i < len(n); i++ {
				customName += toTitle(n[i])
			}
			strct.EnumType = "string"
			strct.Enums = append(strct.Enums, Enum{
				Name:  customName,
				Const: v,
			})
		case int:
			customName += strconv.Itoa(v)
			strct.EnumType = "int"
			strct.Enums = append(strct.Enums, Enum{
				Name:  customName,
				Const: int(v),
			})
		case int32:
			customName += strconv.Itoa(int(v))
			strct.EnumType = "int"
			strct.Enums = append(strct.Enums, Enum{
				Name:  customName,
				Const: int(v),
			})
		case int64:
			customName += strconv.Itoa(int(v))
			strct.EnumType = "int"
			strct.Enums = append(strct.Enums, Enum{
				Name:  customName,
				Const: int(v),
			})
		case float32:
			customName += strconv.FormatFloat(float64(v), 'f', 0, 64)
			strct.EnumType = "int"
			strct.Enums = append(strct.Enums, Enum{
				Name:  customName,
				Const: int(v),
			})
		case float64:
			customName += strconv.FormatFloat(v, 'f', 0, 64)
			strct.EnumType = "int"
			strct.Enums = append(strct.Enums, Enum{
				Name:  customName,
				Const: int(v),
			})
		}

	}

	g.Structs[strct.Name] = strct

	return name, nil
}

func toTitle(s string) string {
	r := []rune(s)
	for idx, val := range r {
		if idx == 0 {
			r[idx] = unicode.ToUpper(val)
		} else {
			r[idx] = unicode.ToLower(val)
		}
	}
	return string(r)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func getPrimitiveTypeName(schemaType string, subType string) (name string, err error) {
	switch schemaType {
	case "array":
		if subType == "" {
			return "error_creating_array", errors.New("can't create an array of an empty subtype")
		}
		return "[]" + subType, nil
	case "boolean":
		return "bool", nil
	case "integer":
		return "int", nil
	case "number":
		return "float64", nil
	case "null":
		return "nil", nil
	case "object":
		if subType == "" {
			return "error_creating_object", errors.New("can't create an object of an empty subtype")
		}
		return subType, nil
	case "string":
		return "string", nil
	case "time":
		return "time.Time", nil
	}

	return "undefined", fmt.Errorf("failed to get a primitive type for schemaType %s and subtype %s",
		schemaType, subType)
}

// return a name for this (sub-)schema.
func (g *Generator) getSchemaName(keyName string, schema *Schema) string {
	if len(schema.Title) > 0 {
		return getGolangName(schema.Title)
	}
	if keyName != "" {
		return getGolangName(keyName)
	}
	if schema.Parent == nil {
		return "Root"
	}
	if schema.JSONKey != "" {
		return getGolangName(schema.JSONKey)
	}
	if schema.Parent != nil && schema.Parent.JSONKey != "" {
		return getGolangName(schema.Parent.JSONKey + "Item")
	}
	g.anonCount++
	return fmt.Sprintf("Anonymous%d", g.anonCount)
}

// getGolangName strips invalid characters out of golang struct or field names.
func getGolangName(s string) string {
	buf := bytes.NewBuffer([]byte{})
	for i, v := range splitOnAll(s, isNotAGoNameCharacter) {
		if i == 0 && strings.IndexAny(v, "0123456789") == 0 {
			// Go types are not allowed to start with a number, lets prefix with an underscore.
			buf.WriteRune('_')
		}
		buf.WriteString(capitaliseFirstLetter(v))
	}
	return buf.String()
}

func splitOnAll(s string, shouldSplit func(r rune) bool) []string {
	rv := []string{}
	buf := bytes.NewBuffer([]byte{})
	for _, c := range s {
		if shouldSplit(c) {
			rv = append(rv, buf.String())
			buf.Reset()
		} else {
			buf.WriteRune(c)
		}
	}
	if buf.Len() > 0 {
		rv = append(rv, buf.String())
	}
	return rv
}

func isNotAGoNameCharacter(r rune) bool {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return false
	}
	return true
}

func capitaliseFirstLetter(s string) string {
	if s == "" {
		return s
	}
	prefix := s[0:1]
	suffix := s[1:]
	return strings.ToUpper(prefix) + suffix
}

// Struct defines the data required to generate a struct in Go.
type Struct struct {
	// The ID within the JSON schema, e.g. #/$defs/address
	ID string
	// The golang name, e.g. "Address"
	Name string
	// Description of the struct
	Description string
	Fields      map[string]Field

	Func Func

	Enums    []Enum
	EnumType string

	GenerateCode   bool
	AdditionalType string
}

type Func struct {
	Name      string
	NameTypes []string
}

type Enum struct {
	Name  string
	Const any
}

// Field defines the data required to generate a field in Go.
type Field struct {
	// The golang name, e.g. "Address1"
	Name string
	// The JSON name, e.g. "address1"
	JSONName string
	// The golang type of the field, e.g. a built-in type like "string" or the name of a struct generated
	// from the JSON schema.
	Type string
	// Required is set to true when the field is required.
	Required    bool
	Description string
}
