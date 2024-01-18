// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"bytes"
	"io"

	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"gopkg.in/yaml.v3"
)

// ReadFileFromBlob reads a file from a blob and returns the content
func ReadFileFromBlob(blob *git.Blob) ([]byte, error) {
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
	return buf, nil
}

// ReadYAMLFromBlob reads a yaml file from a blob and unmarshals it
func ReadYAMLFromBlob(blob *git.Blob) (map[string]interface{}, error) {
	buf, err := ReadFileFromBlob(blob)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(buf, &result); err != nil {
		log.Error("yaml.Unmarshal: %v", err)
		return nil, err
	}
	if result != nil {
		for k, v := range result {
			if val, err := ToStringKeys(v); err != nil {
				log.Error("ToStringKeys: %v", err)
			} else {
				(result)[k] = val
			}
		}
	}
	return result, nil
}

// ReadJSONFromBlob reads a json file from a blob and unmarshals it
func ReadJSONFromBlob(blob *git.Blob) (map[string]interface{}, error) {
	buf, err := ReadFileFromBlob(blob)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
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
