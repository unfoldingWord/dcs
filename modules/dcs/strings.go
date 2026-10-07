// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

import (
	"errors"
	"fmt"
	"strings"
)

// StringHasSuffix returns bool if str ends in the suffix
func StringHasSuffix(str, suffix string) bool {
	return strings.HasSuffix(str, suffix)
}

// ToStringKeys takes an interface and change it to map[string]interface{} on all levels
func ToStringKeys(val any) (any, error) {
	var err error
	switch val := val.(type) {
	case map[any]any:
		m := make(map[string]any)
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
	case map[string]any:
		m := make(map[string]any)
		for k, v := range val {
			m[k], err = ToStringKeys(v)
			if err != nil {
				return nil, err
			}
		}
		return m, nil
	case []any:
		l := make([]any, len(val))
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

// GetColorFromString gets a hexadecimal number for a color based on string
func GetColorFromString(str string) string {
	hash := 0
	for i := range len(str) {
		hash = int(str[i]) + ((hash << 5) - hash)
	}
	var sb strings.Builder
	sb.WriteString("#")
	for i := range 3 {
		value := (hash >> (i * 8)) & 0xFF
		fmt.Fprintf(&sb, "%02x", value)
	}
	return sb.String()
}
