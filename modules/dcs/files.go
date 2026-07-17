// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"bytes"
	"context"
	"io"

	"gitea.dev/modules/charset"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"

	"gopkg.in/yaml.v3"
)

// ReadFileFromBlob reads a file from a blob and returns the content
func ReadFileFromBlob(ctx context.Context, blob *git.Blob) ([]byte, error) {
	dataRc, err := blob.DataAsync(ctx)
	if err != nil {
		log.Warn("DataAsync Error: %v\n", err)
		return nil, err
	}
	defer dataRc.Close()

	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(dataRc, buf)
	buf = buf[:n]

	rd := charset.ToUTF8WithFallbackReader(io.MultiReader(bytes.NewReader(buf), dataRc), charset.ConvertOpts{})
	buf, err = io.ReadAll(rd)
	if err != nil {
		log.Error("io.ReadAll: %v", err)
		return nil, err
	}
	return buf, nil
}

// ReadYAMLFromBlob reads a yaml file from a blob and unmarshals it
func ReadYAMLFromBlob(ctx context.Context, blob *git.Blob) (map[string]any, error) {
	buf, err := ReadFileFromBlob(ctx, blob)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := yaml.Unmarshal(buf, &result); err != nil {
		log.Error("yaml.Unmarshal: %v", err)
		return nil, err
	}
	for k, v := range result {
		if val, err := ToStringKeys(v); err != nil {
			log.Error("ToStringKeys: %v", err)
		} else {
			result[k] = val
		}
	}
	return result, nil
}

// ReadJSONFromBlob reads a json file from a blob and unmarshals it
func ReadJSONFromBlob(ctx context.Context, blob *git.Blob) (map[string]any, error) {
	buf, err := ReadFileFromBlob(ctx, blob)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err = json.Unmarshal(buf, &result); err != nil {
		log.Error("json.Unmarshal: %v", err)
		return nil, err
	}

	for k, v := range result {
		if val, err := ToStringKeys(v); err != nil {
			log.Error("ToStringKeys: %v", err)
		} else {
			(result)[k] = val
		}
	}
	return result, nil
}
