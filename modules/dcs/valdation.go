// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"github.com/santhosh-tekuri/jsonschema/v5"
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
	if err := ValidateJSONFromBlob(entry.Blob()); err != nil {
		log.Warn("Error decoding JSON file %s: %v\n", entry.Name(), err)
		return fmt.Sprintf("Error reading JSON file %s: %s\n", entry.Name(), err.Error())
	}
	return ""
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
		if len(valErr.Causes) > 0 {
			str += "* <root>:\n"
		}
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
