// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
//
// Allow "encoding/json" import

package base

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
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
	case *json.SyntaxError:
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

// ValidateManifestFile validates a manifest file and returns the results as a string
func ValidateManifestFile(entry *git.TreeEntry) string {
	var result *jsonschema.ValidationError
	if entry != nil {
		if r, err := ValidateManifestTreeEntry(entry); err != nil {
			fmt.Printf("ValidateManifestTreeEntry: %v\n", err)
		} else {
			result = r
		}
	}
	return StringifyValidationError(result)
}

// StringHasSuffix returns bool if str ends in the suffix
func StringHasSuffix(str, suffix string) bool {
	return strings.HasSuffix(str, suffix)
}

// ValidateManifestTreeEntry validates a tree entry that is a manifest file and returns the results
func ValidateManifestTreeEntry(entry *git.TreeEntry) (*jsonschema.ValidationError, error) {
	manifest, err := ReadYAMLFromBlob(entry.Blob())
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return ValidateBlobByRC020Schema(manifest)
}

// StringifyValidationError returns a semi-colon & new line separated string of the validation errors
func StringifyValidationError(valErr *jsonschema.ValidationError) string {
	return stringifyValidationError(valErr, "")
}

func stringifyValidationError(valErr *jsonschema.ValidationError, padding string) string {
	if valErr == nil {
		return ""
	}
	str := ""
	loc := "/"
	message := ""
	if valErr.InstanceLocation != "" {
		loc = valErr.InstanceLocation
	}
	if padding != "" {
		str = "\n"
		message = valErr.Message
	}
	str += fmt.Sprintf("%s * %s: %s", padding, loc, message)
	for _, cause := range valErr.Causes {
		str += stringifyValidationError(cause, padding+"  ")
	}
	return str
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
