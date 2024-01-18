// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"

	"github.com/santhosh-tekuri/jsonschema/v5"

	_ "github.com/santhosh-tekuri/jsonschema/v5/httploader" // Loader for Schema via HTTP
)

var rc02Schema *jsonschema.Schema

// GetRC02Schema returns the schema for RC v0.2
func GetRC02Schema(reload bool) (*jsonschema.Schema, error) {
	githubPrefix := "https://raw.githubusercontent.com/unfoldingWord/rc-schema/master/"
	if rc02Schema == nil || reload {
		jsonschema.Loaders["https"] = func(url string) (io.ReadCloser, error) {
			res, err := http.Get(url)
			if err == nil && res != nil && res.StatusCode == 200 {
				return res.Body, nil
			}
			log.Warn("GetRC02Schema: not able to get the schema file remotely [%q]: %v", url, err)
			uriPath := strings.TrimPrefix(url, githubPrefix)
			fileBuf, err := options.AssetFS().ReadFile("schema", "rc02", uriPath)
			if err != nil {
				log.Error("GetRC02Schema: local schema file not found: [options/schema/rc02/%s]: %v", uriPath, err)
				return nil, err
			}
			return io.NopCloser(bytes.NewReader(fileBuf)), nil
		}
		var err error
		rc02Schema, err = jsonschema.Compile(githubPrefix + "rc.schema.json")
		if err != nil {
			return nil, err
		}
	}
	return rc02Schema, nil
}

// ValidateMapByRC02Schema Validates a map structure by the RC v0.2.0 schema and returns the result
func ValidateMapByRC02Schema(data map[string]interface{}) (*jsonschema.ValidationError, error) {
	if data == nil {
		return &jsonschema.ValidationError{Message: "file cannot be empty"}, nil
	}
	schema, err := GetRC02Schema(false)
	if err != nil {
		return nil, err
	}
	if err = schema.Validate(data); err != nil {
		switch e := err.(type) {
		case *jsonschema.ValidationError:
			return e, nil
		default:
			return nil, e
		}
	}
	return nil, nil
}
