// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dcs

// The manifest and langnames maps DCS parses come from user-supplied YAML/JSON,
// so a key can be missing or hold an unexpected type even when a schema check
// passed earlier. These accessors yield the zero value instead of panicking.

// MapStr returns m[key] as a string, or "" if absent or not a string.
func MapStr(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// MapBool returns m[key] as a bool, or false if absent or not a bool.
func MapBool(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}

// MapMap returns m[key] as a nested map, or nil if absent or not a map.
func MapMap(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	return v
}

// MapSlice returns m[key] as a slice, or nil if absent or not a slice.
func MapSlice(m map[string]any, key string) []any {
	v, _ := m[key].([]any)
	return v
}

// MapStrSlice returns m[key] as a []string, or nil if absent or not a []string.
func MapStrSlice(m map[string]any, key string) []string {
	v, _ := m[key].([]string)
	return v
}
