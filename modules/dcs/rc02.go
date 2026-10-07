// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// rc02Schema is the Resource Container 0.2 schema bundled in options/schema/rc02. The
// file keeps its upstream GitHub $id, which is mapped onto that directory (or a
// server's custom/options copy of it) rather than fetched.
var rc02Schema = &localSchema{
	dir:      "rc02",
	idPrefix: "https://raw.githubusercontent.com/unfoldingWord/rc-schema/master/",
	rootFile: "rc.schema.json",
}

// GetRC02Schema returns the schema for RC v0.2, compiled from options/schema/rc02
func GetRC02Schema(reload bool) (*jsonschema.Schema, error) {
	return rc02Schema.Get(reload)
}

// ValidateMapByRC02Schema validates a map structure by the RC v0.2.0 schema and returns the result
func ValidateMapByRC02Schema(data map[string]any) (*jsonschema.ValidationError, error) {
	return rc02Schema.Validate(data)
}
