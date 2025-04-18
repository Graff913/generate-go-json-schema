package generate

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
)

func getOrderedFieldNames(m map[string]Field) []string {
	keys := make([]string, len(m))
	idx := 0
	for k := range m {
		keys[idx] = k
		idx++
	}
	sort.Strings(keys)
	return keys
}

func getOrderedStructNames(m map[string]Struct) []string {
	keys := make([]string, len(m))
	idx := 0
	for k := range m {
		keys[idx] = k
		idx++
	}
	sort.Strings(keys)
	return keys
}

// Output generates code and writes to w.
func Output(w io.Writer, g *Generator, pkg string, bson bool, tagOmitempty bool) {
	structs := g.Structs
	aliases := g.Aliases

	fmt.Fprintln(w, "// Code generated by schema-generate. DO NOT EDIT.")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "package %v\n", cleanPackageName(pkg))

	// write all the code into a buffer, compiler functions will return list of imports
	// write list of imports into main output stream, followed by the code
	codeBuf := new(bytes.Buffer)
	imports := make(map[string]bool)
	if bson {
		imports["go.mongodb.org/mongo-driver/bson/primitive"] = true
	}
	for _, v := range structs {
		for _, f := range v.Fields {
			if f.Type == "*time.Time" || f.Type == "time.Time" {
				imports["time"] = true
			}
		}
	}

	//for _, k := range getOrderedStructNames(structs) {
	//	s := structs[k]
	//	if s.GenerateCode {
	//		emitMarshalCode(codeBuf, s, imports)
	//		emitUnmarshalCode(codeBuf, s, imports)
	//	}
	//}

	if len(imports) > 0 {
		fmt.Fprintf(w, "\nimport (\n")
		for k := range imports {
			fmt.Fprintf(w, "    \"%s\"\n", k)
		}
		fmt.Fprintf(w, ")\n")
	}

	_ = aliases

	//for _, k := range getOrderedFieldNames(aliases) {
	//	a := aliases[k]
	//
	//	fmt.Fprintln(w, "")
	//	fmt.Fprintf(w, "// %s\n", a.Name)
	//	fmt.Fprintf(w, "type %s %s\n", a.Name, a.Type)
	//}

	for _, k := range getOrderedStructNames(structs) {
		s := structs[k]

		fmt.Fprintln(w, "")
		outputNameAndDescriptionComment(s.Name, s.Description, w)
		if len(s.Enums) > 0 {

			fmt.Fprintf(w, "type %s %s\n", s.Name, s.EnumType)
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "const (")
			for _, val := range s.Enums {
				if s.EnumType == "string" {
					fmt.Fprintf(w, "\t%s %s = \"%s\"\n", val.Name, s.Name, val.Const)
				} else {
					fmt.Fprintf(w, "\t%s %s = %d\n", val.Name, s.Name, val.Const)
				}
			}
			fmt.Fprintln(w, ")")
		} else if s.Func.Name != "" {
			fmt.Fprintf(w, "type %s interface {\n", s.Name)

			fmt.Fprintf(w, "  %s() bool \n", s.Func.Name)

			fmt.Fprintln(w, "}")

			for _, val := range s.Func.NameTypes {
				fmt.Fprintln(w, "")
				fmt.Fprintf(w, "func (d *%s) %s() bool {\n", val, s.Func.Name)
				fmt.Fprintf(w, "  return true\n")
				fmt.Fprintln(w, "}")
			}
		} else {
			fmt.Fprintf(w, "type %s struct {\n", s.Name)
			for _, fieldKey := range getOrderedFieldNames(s.Fields) {
				f := s.Fields[fieldKey]
				//link := "*"
				//if f.Required {
				//	link = ""
				//}
				// Only apply omitempty if the field is not required.
				omitempty := ",omitempty"
				if tagOmitempty || f.Required {
					omitempty = ""
				}
				bsonTag := ""
				if bson {
					bsonTag = fmt.Sprintf(" bson:\"%s%s\"", f.JSONName, omitempty)
				}
				if f.Description != "" {
					outputFieldDescriptionComment(f.Description, w)
				}
				fmt.Fprintf(w, "  %s %s `json:\"%s%s\"%s`\n", f.Name, f.Type, f.JSONName, omitempty, bsonTag)
			}
			fmt.Fprintln(w, "}")
		}

	}

	// write code after structs for clarity
	w.Write(codeBuf.Bytes())
}

func emitMarshalCode(w io.Writer, s Struct, imports map[string]bool) {
	imports["bytes"] = true
	fmt.Fprintf(w,
		`
func (strct *%s) MarshalJSON() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0))
	buf.WriteString("{")
`, s.Name)

	if len(s.Fields) > 0 {
		fmt.Fprintf(w, "    comma := false\n")
		// Marshal all the defined fields
		for _, fieldKey := range getOrderedFieldNames(s.Fields) {
			f := s.Fields[fieldKey]
			if f.JSONName == "-" {
				continue
			}
			if f.Required {
				fmt.Fprintf(w, "    // \"%s\" field is required\n", f.Name)
				// currently only objects are supported
				if strings.HasPrefix(f.Type, "*") {
					imports["errors"] = true
					fmt.Fprintf(w, `    if strct.%s == nil {
        return nil, errors.New("%s is a required field")
    }
`, f.Name, f.JSONName)
				} else {
					fmt.Fprintf(w, "    // only required object types supported for marshal checking (for now)\n")
				}
			}

			fmt.Fprintf(w,
				`    // Marshal the "%[1]s" field
    if comma { 
        buf.WriteString(",") 
    }
    buf.WriteString("\"%[1]s\": ")
	if tmp, err := json.Marshal(strct.%[2]s); err != nil {
		return nil, err
 	} else {
 		buf.Write(tmp)
	}
	comma = true
`, f.JSONName, f.Name)
		}
	}
	if s.AdditionalType != "" {
		if s.AdditionalType != "false" {
			imports["fmt"] = true

			if len(s.Fields) == 0 {
				fmt.Fprintf(w, "    comma := false\n")
			}

			fmt.Fprintf(w, "    // Marshal any additional Properties\n")
			// Marshal any additional Properties
			fmt.Fprintf(w, `    for k, v := range strct.AdditionalProperties {
		if comma {
			buf.WriteString(",")
		}
        buf.WriteString(fmt.Sprintf("\"%%s\":", k))
		if tmp, err := json.Marshal(v); err != nil {
			return nil, err
		} else {
			buf.Write(tmp)
		}
        comma = true
	}
`)
		}
	}

	fmt.Fprintf(w, `
	buf.WriteString("}")
	rv := buf.Bytes()
	return rv, nil
}
`)
}

func emitUnmarshalCode(w io.Writer, s Struct, imports map[string]bool) {
	imports["encoding/json"] = true
	// unmarshal code
	fmt.Fprintf(w, `
func (strct *%s) UnmarshalJSON(b []byte) error {
`, s.Name)
	// setup required bools
	for _, fieldKey := range getOrderedFieldNames(s.Fields) {
		f := s.Fields[fieldKey]
		if f.Required {
			fmt.Fprintf(w, "    %sReceived := false\n", f.JSONName)
		}
	}
	// setup initial unmarshal
	fmt.Fprintf(w, `    var jsonMap map[string]json.RawMessage
    if err := json.Unmarshal(b, &jsonMap); err != nil {
        return err
    }`)

	// figure out if we need the "v" output of the range keyword
	needVal := "_"
	if len(s.Fields) > 0 || s.AdditionalType != "false" {
		needVal = "v"
	}
	// start the loop
	fmt.Fprintf(w, `
    // parse all the defined properties
    for k, %s := range jsonMap {
        switch k {
`, needVal)
	// handle defined properties
	for _, fieldKey := range getOrderedFieldNames(s.Fields) {
		f := s.Fields[fieldKey]
		if f.JSONName == "-" {
			continue
		}
		fmt.Fprintf(w, `        case "%s":
            if err := json.Unmarshal([]byte(v), &strct.%s); err != nil {
                return err
             }
`, f.JSONName, f.Name)
		if f.Required {
			fmt.Fprintf(w, "            %sReceived = true\n", f.JSONName)
		}
	}

	// handle additional property
	if s.AdditionalType != "" {
		if s.AdditionalType == "false" {
			// all unknown properties are not allowed
			imports["fmt"] = true
			fmt.Fprintf(w, `        default:
            return fmt.Errorf("additional property not allowed: \"" + k + "\"")
`)
		} else {
			fmt.Fprintf(w, `        default:
            // an additional "%s" value
            var additionalValue %s
            if err := json.Unmarshal([]byte(v), &additionalValue); err != nil {
                return err // invalid additionalProperty
            }
            if strct.AdditionalProperties == nil {
                strct.AdditionalProperties = make(map[string]%s, 0)
            }
            strct.AdditionalProperties[k]= additionalValue
`, s.AdditionalType, s.AdditionalType, s.AdditionalType)
		}
	}
	fmt.Fprintf(w, "        }\n") // switch
	fmt.Fprintf(w, "    }\n")     // for

	// check all Required fields were received
	for _, fieldKey := range getOrderedFieldNames(s.Fields) {
		f := s.Fields[fieldKey]
		if f.Required {
			imports["errors"] = true
			fmt.Fprintf(w, `    // check if %s (a required property) was received
    if !%sReceived {
        return errors.New("\"%s\" is required but was not present")
    }
`, f.JSONName, f.JSONName, f.JSONName)
		}
	}

	fmt.Fprintf(w, "    return nil\n")
	fmt.Fprintf(w, "}\n") // UnmarshalJSON
}

func outputNameAndDescriptionComment(name, description string, w io.Writer) {
	if strings.Index(description, "\n") == -1 {
		fmt.Fprintf(w, "// %s %s\n", name, description)
		return
	}

	dl := strings.Split(description, "\n")
	fmt.Fprintf(w, "// %s %s\n", name, strings.Join(dl, "\n// "))
}

func outputFieldDescriptionComment(description string, w io.Writer) {
	if strings.Index(description, "\n") == -1 {
		fmt.Fprintf(w, "\n  // %s\n", description)
		return
	}

	dl := strings.Split(description, "\n")
	fmt.Fprintf(w, "\n  // %s\n", strings.Join(dl, "\n  // "))
}

func cleanPackageName(pkg string) string {
	pkg = strings.Replace(pkg, ".", "", -1)
	pkg = strings.Replace(pkg, "-", "", -1)
	return pkg
}
