// The schema-generate binary reads the JSON schema files passed as arguments
// and outputs the corresponding Go structs.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Graff913/generate-go-json-schema"
)

var (
	o                     = flag.String("o", "", "The output file for the schema.")
	p                     = flag.String("p", "main", "The package that the structs are created in.")
	bson                  = flag.Bool("bson", false, "Generate bson tags")
	omitempty             = flag.Bool("omitempty", false, "Generate omitempty tags")
	rootPath              = flag.String("r", "", "The root path repo")
	i                     = flag.String("i", "", "A single file path (used for backwards compatibility).")
	schemaKeyRequiredFlag = flag.Bool("schemaKeyRequired", false, "Allow input files with no $schema key.")
)

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		_, _ = fmt.Fprintln(os.Stderr, "  paths")
		_, _ = fmt.Fprintln(os.Stderr, "\tThe input JSON Schema files.")
	}

	flag.Parse()

	inputFiles := flag.Args()
	if *i != "" {
		inputFiles = append(inputFiles, *i)
	}
	if len(inputFiles) == 0 {
		_, _ = fmt.Fprintln(os.Stderr, "No input JSON Schema files.")
		flag.Usage()
		os.Exit(1)
	}

	analysisFiles, err := generate.AnalysisFiles(*rootPath, inputFiles)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	schemas, err := generate.ReadInputFiles(analysisFiles, *schemaKeyRequiredFlag)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	g := generate.New(schemas...)

	err = g.CreateTypes(*rootPath, *p, *bson)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Failure generating structs: ", err)
		os.Exit(1)
	}

	var w io.Writer = os.Stdout

	if *o != "" {
		w, err = os.Create(*o)

		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "Error opening output file: ", err)
			return
		}
	}

	generate.Output(w, g, *p, *bson, *omitempty)
}
