// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"gitea.dev/modules/charset"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// ValidateYAMLFile validates a yaml file
func ValidateYAMLFile(ctx context.Context, gitRepo *git.Repository, entry *git.TreeEntry) string {
	if _, err := ReadYAMLFromBlob(ctx, entry.Blob(gitRepo)); err != nil {
		return strings.ReplaceAll(err.Error(), " converting YAML to JSON", "")
	}
	return ""
}

// ValidateJSONFile validates a json file
func ValidateJSONFile(ctx context.Context, gitRepo *git.Repository, entry *git.TreeEntry) string {
	if err := ValidateJSONFromBlob(ctx, entry.Blob(gitRepo)); err != nil {
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
	var sb strings.Builder
	sb.WriteString(padding)
	if parentErr == nil {
		fmt.Fprintf(&sb, "Invalid: %s\n", strings.TrimSuffix(valErr.Message, "#"))
		if len(valErr.Causes) > 0 {
			sb.WriteString("* <root>:\n")
		}
	} else {
		loc := ""
		if valErr.InstanceLocation != "" {
			loc = strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(valErr.InstanceLocation, parentErr.InstanceLocation), "/"), "/", ".")
			if loc != "" {
				loc = strings.TrimPrefix(loc, "/") + ": "
			}
		}
		fmt.Fprintf(&sb, "* %s%s\n", loc, valErr.Message)
	}
	sort.Slice(valErr.Causes, func(i, j int) bool { return valErr.Causes[i].InstanceLocation < valErr.Causes[j].InstanceLocation })
	for _, cause := range valErr.Causes {
		sb.WriteString(convertValidationErrorToString(cause, valErr, padding+"  "))
	}
	return sb.String()
}

// ValidateJSONFromBlob reads a json file from a blob and unmarshals it returning any errors
func ValidateJSONFromBlob(ctx context.Context, blob *git.Blob) error {
	dataRc, err := blob.DataAsync(ctx)
	if err != nil {
		log.Warn("DataAsync Error: %v\n", err)
		return err
	}
	defer dataRc.Close()

	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(dataRc, buf)
	buf = buf[:n]

	rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc), charset.ConvertOpts{})
	buf, err = io.ReadAll(rd)
	if err != nil {
		log.Error("io.ReadAll: %v", err)
		return err
	}

	var result any
	err = json.Unmarshal(buf, &result)
	if err != nil {
		log.Error("json.Unmarshal: %v", err)
	}
	return err
}
