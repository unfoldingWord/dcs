// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"bufio"
	"bytes"
	json_package "encoding/json" //nolint:depguard
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/util"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v2"
)

// ValidateYAMLFile validates a yaml file
func ValidateYAMLFile(entry *git.TreeEntry) string {
	if _, err := ReadYAMLFromBlob(entry.Blob()); err != nil {
		return strings.ReplaceAll(err.Error(), " converting YAML to JSON", "")
	}
	return ""
}

// ValidateJSONFile validates a json file
func ValidateJSONFile(entry *git.TreeEntry) string {
	validationErr := ValidateJSONFromBlob(entry.Blob())
	if validationErr == nil {
		return ""
	}
	// JSON is not valid so we need to get all the errors
	dataRc, err := entry.Blob().DataAsync()
	if err != nil {
		log.Error("DataAsync Error: %v\n", err)
		return fmt.Sprintf("Error reading JSON file: %s\n", validationErr.Error())
	}
	defer dataRc.Close()

	buf := make([]byte, 1024)
	n, err := util.ReadAtMost(dataRc, buf)
	if err != nil {
		log.Error("util.ReadAtMost Error: %v\n", err)
		return fmt.Sprintf("Error reading JSON file: %s\n", validationErr.Error())
	}
	buf = buf[:n]

	rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc))
	buf, err = io.ReadAll(rd)
	if err != nil {
		log.Error("io.ReadAll: %v", err)
		return fmt.Sprintf("Error reading JSON file: %s", validationErr.Error())
	}

	switch err := validationErr.(type) {
	case *json_package.SyntaxError:
		var errors string
		scanner := bufio.NewScanner(strings.NewReader(string(buf)))
		var line int
		var readBytes int64
		for scanner.Scan() {
			// +1 for the \n character
			readBytes += int64(len(scanner.Bytes()) + 1)
			line++
			if readBytes >= err.Offset {
				errors += fmt.Sprintf("error: json: line %d: %s\n", line, err.Error())
			}
		}
		return errors
	default:
		log.Warn("Error decoding JSON: %v\n", validationErr)
		return fmt.Sprintf("Error decoding JSON: %s\n", validationErr.Error())
	}
}

// StringHasSuffix returns bool if str ends in the suffix
func StringHasSuffix(str, suffix string) bool {
	return strings.HasSuffix(str, suffix)
}

// ValidateManifestFileAsHTML validates a manifest file and returns the results as template.HTML
func ValidateManifestFileAsHTML(entry *git.TreeEntry) template.HTML {
	var result *jsonschema.ValidationError
	if r, err := ValidateManifestTreeEntry(entry); err != nil {
		log.Warn("ValidateManifestTreeEntry: %v\n", err)
	} else {
		result = r
	}
	return template.HTML(ConvertValidationErrorToHTML(result))
}

// ValidateManifestTreeEntry validates a tree entry that is a manifest file and returns the results
func ValidateManifestTreeEntry(entry *git.TreeEntry) (*jsonschema.ValidationError, error) {
	if entry == nil {
		return nil, nil
	}
	manifest, err := ReadYAMLFromBlob(entry.Blob())
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return ValidateBlobByRC020Schema(manifest)
}

// ConvertValidationErrorToString returns a semi-colon & new line separated string of the validation errors
func ConvertValidationErrorToString(valErr *jsonschema.ValidationError) string {
	return convertValidationErrorToString(valErr, nil, "")
}

func convertValidationErrorToString(valErr, parentErr *jsonschema.ValidationError, padding string) string {
	if valErr == nil {
		return ""
	}
	str := padding
	if parentErr == nil {
		str += fmt.Sprintf("Invalid: %s\n", strings.TrimSuffix(valErr.Message, "#"))
		str += "* <root>:\n"
	} else {
		loc := ""
		if valErr.InstanceLocation != "" {
			loc = strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(valErr.InstanceLocation, parentErr.InstanceLocation), "/"), "/", ".")
			if loc != "" {
				loc = fmt.Sprintf("%s: ", strings.TrimPrefix(loc, "/"))
			}
		}
		str += fmt.Sprintf("* %s%s\n", loc, valErr.Message)
	}
	sort.Slice(valErr.Causes, func(i, j int) bool { return valErr.Causes[i].InstanceLocation < valErr.Causes[j].InstanceLocation })
	for _, cause := range valErr.Causes {
		str += convertValidationErrorToString(cause, valErr, padding+"  ")
	}
	return str
}

// ConvertValidationErrorToHTML converts a validation error object to an HTML string
func ConvertValidationErrorToHTML(valErr *jsonschema.ValidationError) string {
	return convertValidationErrorToHTML(valErr, nil)
}

func convertValidationErrorToHTML(valErr, parentErr *jsonschema.ValidationError) string {
	if valErr == nil {
		return ""
	}
	var html string
	if parentErr == nil {
		html += fmt.Sprintf("<strong>Invalid:</strong> %s\n", strings.TrimSuffix(valErr.Message, "#"))
		html += "<ul>\n"
		html += "<li><strong>&lt;root&gt;:</strong></li>\n"
	} else {
		html += "<ul>\n"
		loc := ""
		if valErr.InstanceLocation != "" {
			loc = strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(valErr.InstanceLocation, parentErr.InstanceLocation), "/"), "/", ".")
			if loc != "" {
				loc = fmt.Sprintf("<strong>%s:</strong> ", strings.TrimPrefix(loc, "/"))
			}
		}
		html += fmt.Sprintf("<li>%s%s</li>\n", loc, valErr.Message)
	}
	sort.Slice(valErr.Causes, func(i, j int) bool { return valErr.Causes[i].InstanceLocation < valErr.Causes[j].InstanceLocation })
	for _, cause := range valErr.Causes {
		html += convertValidationErrorToHTML(cause, valErr)
	}
	html += "</ul>\n"
	return html
}

// ValidateBlobByRC020Schema Validates a blob by the RC v0.2.0 schema and returns the result
func ValidateBlobByRC020Schema(manifest *map[string]interface{}) (*jsonschema.ValidationError, error) {
	schemaText, err := GetRC020Schema()
	if err != nil {
		return nil, err
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", strings.NewReader(string(schemaText))); err != nil {
		return nil, err
	}
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, err
	}
	if err = schema.Validate(*manifest); err != nil {
		switch e := err.(type) {
		case *jsonschema.ValidationError:
			return e, nil
		default:
			return nil, e
		}
	}
	return nil, nil
}

// ToStringKeys takes an interface and change it to map[string]interface{} on all levels
func ToStringKeys(val interface{}) (interface{}, error) {
	var err error
	switch val := val.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for k, v := range val {
			k, ok := k.(string)
			if !ok {
				return nil, errors.New("found non-string key")
			}
			m[k], err = ToStringKeys(v)
			if err != nil {
				return nil, err
			}
		}
		return m, nil
	case map[string]interface{}:
		m := make(map[string]interface{})
		for k, v := range val {
			m[k], err = ToStringKeys(v)
			if err != nil {
				return nil, err
			}
		}
		return m, nil
	case []interface{}:
		l := make([]interface{}, len(val))
		for i, v := range val {
			l[i], err = ToStringKeys(v)
			if err != nil {
				return nil, err
			}
		}
		return l, nil
	default:
		return val, nil
	}
}

var rc02Schema []byte

// GetRC020Schema Returns the schema for RC v0.2, first trying the online URL, then from file if not already done
func GetRC020Schema() ([]byte, error) {
	rc02SchmeFileName := "rc.schema.json"
	schemaOnlineURL := "https://raw.githubusercontent.com/unfoldingWord/rc-schema/master/" + rc02SchmeFileName
	if rc02Schema == nil {
		var err error
		if res, err := http.Get(schemaOnlineURL); err == nil {
			defer res.Body.Close()
			// read all
			if body, err := io.ReadAll(res.Body); err == nil {
				rc02Schema = body
			}
		}
		if rc02Schema == nil {
			// Failed to get schema online, falling back to the one in the options dir
			if rc02Schema, err = options.Schemas(rc02SchmeFileName); err != nil {
				return nil, err
			}
		}
	}
	return rc02Schema, nil
}

// ReadYAMLFromBlob reads a yaml file from a blob and unmarshals it
func ReadYAMLFromBlob(blob *git.Blob) (*map[string]interface{}, error) {
	dataRc, err := blob.DataAsync()
	if err != nil {
		log.Warn("DataAsync Error: %v\n", err)
		return nil, err
	}
	defer dataRc.Close()

	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(dataRc, buf)
	buf = buf[:n]

	rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc))
	buf, err = io.ReadAll(rd)
	if err != nil {
		log.Error("io.ReadAll: %v", err)
		return nil, err
	}

	var result *map[string]interface{}
	if err := yaml.Unmarshal(buf, &result); err != nil {
		log.Error("yaml.Unmarshal: %v", err)
		return nil, err
	}
	for k, v := range *result {
		if val, err := ToStringKeys(v); err != nil {
			log.Error("ToStringKeys: %v", err)
		} else {
			(*result)[k] = val
		}
	}
	return result, nil
}

// ValidateJSONFromBlob reads a json file from a blob and unmarshals it returning any errors
func ValidateJSONFromBlob(blob *git.Blob) error {
	dataRc, err := blob.DataAsync()
	if err != nil {
		log.Warn("DataAsync Error: %v\n", err)
		return err
	}
	defer dataRc.Close()

	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(dataRc, buf)
	buf = buf[:n]

	rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc))
	buf, err = io.ReadAll(rd)
	if err != nil {
		log.Error("io.ReadAll: %v", err)
		return err
	}

	var result interface{}
	err = json.Unmarshal(buf, &result)
	if err != nil {
		log.Error("json.Unmarshal: %v", err)
	}
	return err
}
