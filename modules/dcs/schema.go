// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"gitea.dev/modules/options"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// localSchema is a JSON schema compiled from the files under options/schema/<dir>.
// Every $ref is resolved there too, never over the network, so the schema applied is
// always the bundled one or the server's custom/options override of it.
type localSchema struct {
	dir      string // sub-directory of options/schema holding the schema files
	idPrefix string // the $id base the schema files declare; mapped onto dir
	rootFile string // the file to compile, relative to dir

	mu     sync.RWMutex
	schema *jsonschema.Schema
}

// Get returns the compiled schema, compiling it on first use or when reload is set.
// A failed reload keeps the previously compiled schema in place.
func (ls *localSchema) Get(reload bool) (*jsonschema.Schema, error) {
	ls.mu.RLock()
	schema := ls.schema
	ls.mu.RUnlock()
	if schema != nil && !reload {
		return schema, nil
	}

	schema, err := ls.compile()
	if err != nil {
		return nil, err
	}
	ls.mu.Lock()
	ls.schema = schema
	ls.mu.Unlock()
	return schema, nil
}

func (ls *localSchema) compile() (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	compiler.LoadURL = func(url string) (io.ReadCloser, error) {
		rel, ok := strings.CutPrefix(url, ls.idPrefix)
		if !ok {
			return nil, fmt.Errorf("schema %q is not under %s, cannot load it from options/schema/%s", url, ls.idPrefix, ls.dir)
		}
		buf, err := options.AssetFS().ReadFile("schema", ls.dir, rel)
		if err != nil {
			return nil, fmt.Errorf("options/schema/%s/%s: %w", ls.dir, rel, err)
		}
		return io.NopCloser(bytes.NewReader(buf)), nil
	}
	return compiler.Compile(ls.idPrefix + ls.rootFile)
}

// Validate checks data against the schema. A violation comes back as the
// *ValidationError; the error return is reserved for the schema itself being unloadable.
func (ls *localSchema) Validate(data map[string]any) (*jsonschema.ValidationError, error) {
	if data == nil {
		return &jsonschema.ValidationError{Message: "file cannot be empty"}, nil
	}
	schema, err := ls.Get(false)
	if err != nil {
		return nil, err
	}
	if err = schema.Validate(data); err != nil {
		if valErr, ok := errors.AsType[*jsonschema.ValidationError](err); ok {
			return valErr, nil
		}
		return nil, err
	}
	return nil, nil //nolint:nilnil // a nil *ValidationError is the "document is valid" result
}
