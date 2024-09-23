package generate

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
)

type AnalysisFile struct {
	Root bool
	Path string
}

func AnalysisFiles(rootPath string, inputFiles []string) ([]AnalysisFile, error) {
	temp := make(map[string]AnalysisFile)
	for _, file := range inputFiles {
		temp[file] = AnalysisFile{
			Root: true,
			Path: file,
		}

		b, err := os.ReadFile(file)
		if err != nil {
			return nil, errors.New("failed to read the input file with error " + err.Error())
		}

		s := &Schema{}
		err = json.Unmarshal(b, s)
		if err != nil {
			return nil, err
		}

		for _, p := range s.Properties {
			if p.Reference != "" &&
				!strings.HasPrefix(p.Reference, "#") {
				if strings.HasPrefix(p.Reference, "/") {
					path := rootPath + p.Reference
					temp[path] = AnalysisFile{
						Root: false,
						Path: path,
					}
				} else {
					index := strings.LastIndex(file, "/")
					path := file[:index+1] + p.Reference
					temp[path] = AnalysisFile{
						Root: false,
						Path: path,
					}
				}
			}
		}

		for _, d := range s.Definitions {
			for _, p := range d.Properties {
				if p.Reference != "" &&
					!strings.HasPrefix(p.Reference, "#") {
					if strings.HasPrefix(p.Reference, "/") {
						path := rootPath + p.Reference
						temp[path] = AnalysisFile{
							Root: false,
							Path: path,
						}
					} else {
						index := strings.LastIndex(file, "/")
						path := file[:index+1] + p.Reference
						temp[path] = AnalysisFile{
							Root: false,
							Path: path,
						}
					}
				}
			}
		}
	}

	paths := make([]AnalysisFile, 0, len(temp))
	for _, v := range temp {
		paths = append(paths, v)
	}

	return paths, nil
}
