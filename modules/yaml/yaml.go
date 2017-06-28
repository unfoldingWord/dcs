// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package yaml

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"gopkg.in/yaml.v2"
)

var Sanitizer = bluemonday.UGCPolicy()

// IsYamlFile reports whether name looks like a Yaml file
// based on its extension.
func IsYamlFile(name string) bool {
	name = strings.ToLower(name)
	if ".yaml" == filepath.Ext(name) {
		return true
	}
	return false
}

func renderHorizontalHtmlTable(m yaml.MapSlice) (string, error) {
	var err error
	var thead, tbody, table string
	var mi yaml.MapItem
	for _, mi = range m {
		key := mi.Key
		value := mi.Value

		switch slice := key.(type) {
		case yaml.MapSlice:
			key, err = renderHorizontalHtmlTable(slice)
			if err != nil {
				return "", err
			}
		}
		thead += fmt.Sprintf("<th>%v</th>", key)

		switch switchedValue := value.(type) {
		case yaml.MapSlice:
			value, err = renderHorizontalHtmlTable(switchedValue)
			if err != nil {
				return "", err
			}
		case []interface{}:
			v := make([]yaml.MapSlice, len(switchedValue))
			for i, vs := range switchedValue {
				switch vs := vs.(type) {
				case yaml.MapSlice:
					v[i] = vs
				default:
					return "", fmt.Errorf("Unexpected type %T, expected MapSlice", vs)
				}
			}
			value, err = renderVerticalHtmlTable(v)
			if err != nil {
				return "", err
			}
		}
		tbody += fmt.Sprintf("<td>%v</td>", value)
	}

	table = ""
	if len(thead) > 0 {
		table = fmt.Sprintf(`<table data="yaml-metadata"><thead><tr>%s</tr></thead><tbody><tr>%s</tr></table>`, thead, tbody)
	}
	return table, nil
}

func renderVerticalHtmlTable(m []yaml.MapSlice) (string, error) {
	var err error
	var ms yaml.MapSlice
	var mi yaml.MapItem
	var table string

	for _, ms = range m {
		table += `<table data="yaml-metadata">`
		for _, mi = range ms {
			key := mi.Key
			value := mi.Value

			table += `<tr>`
			switch switchedKey := key.(type) {
			case yaml.MapSlice:
				key, err = renderHorizontalHtmlTable(switchedKey)
				if err != nil {
					return "", err
				}
			case []interface{}:
				var ks string
				for _, ki := range switchedKey {
					switch ki := ki.(type) {
					case yaml.MapSlice:
						horiz, err := renderHorizontalHtmlTable(ki)
						if err != nil {
							return "", err
						}
						ks += horiz
					default:
						return "", fmt.Errorf("Unexpected type %T, expected MapSlice", ki)
					}
				}
				key = ks
			}
			table += fmt.Sprintf("<td>%v</td>", key)

			switch switchedValue := value.(type) {
			case yaml.MapSlice:
				value, err = renderHorizontalHtmlTable(switchedValue)
				if err != nil {
					return "", err
				}
			case []interface{}:
				v := make([]yaml.MapSlice, len(switchedValue))
				for i, vs := range switchedValue {
					switch vs := vs.(type) {
					case yaml.MapSlice:
						v[i] = vs
					default:
						return "", fmt.Errorf("Unexpected type %T, expected MapSlice", vs)
					}
				}
				value, err = renderVerticalHtmlTable(v)
				if err != nil {
					return "", err
				}
			}

			switch key {
			case "slug":
				value = fmt.Sprintf(`<a href="content/%v.md">%v</a>`, value, value)
			case "link":
				value = fmt.Sprintf(`<a href="%v/01.md">%v</a>`, value, value)
			}
			table += fmt.Sprintf("<td>%v</td>", value)
			table += `</tr>`
		}
		table += "</table>"
	}

	return table, nil
}

func RenderYaml(data []byte) ([]byte, error) {
	mss := []yaml.MapSlice{}

	if len(data) < 1 {
		return data, nil
	}

	if err := yaml.Unmarshal(data, &mss); err != nil {
		ms := yaml.MapSlice{}
		if err := yaml.Unmarshal(data, &ms); err != nil {
			return nil, err
		}
		table, err := renderHorizontalHtmlTable(ms)
		return []byte(table), err
	}
	table, err := renderVerticalHtmlTable(mss)
	return []byte(table), err
}

func RenderMarkdownYaml(data []byte) ([]byte, error) {
	mss := []yaml.MapSlice{}

	if len(data) < 1 {
		return []byte(""), nil
	}

	lines := strings.Split(string(data), "\r\n")
	if len(lines) == 1 {
		lines = strings.Split(string(data), "\n")
	}
	if len(lines) < 1 || lines[0] != "---" {
		return []byte(""), nil
	}

	if err := yaml.Unmarshal(data, &mss); err != nil {
		ms := yaml.MapSlice{}
		if err := yaml.Unmarshal(data, &ms); err != nil {
			return []byte(""), nil
		}
		table, err := renderHorizontalHtmlTable(ms)
		return []byte(table), err
	}
	table, err := renderVerticalHtmlTable(mss)
	return []byte(table), err
}

func StripYamlFromText(data []byte) []byte {
	mss := []yaml.MapSlice{}
	if err := yaml.Unmarshal(data, &mss); err != nil {
		ms := yaml.MapSlice{}
		if err := yaml.Unmarshal(data, &ms); err != nil {
			return data
		}
	}

	lines := strings.Split(string(data), "\r\n")
	if len(lines) == 1 {
		lines = strings.Split(string(data), "\n")
	}
	if len(lines) < 1 || lines[0] != "---" {
		return data
	}
	body := ""
	atBody := false
	for i, line := range lines {
		if i == 0 {
			continue
		}
		if line == "---" {
			atBody = true
		} else if atBody {
			body += line + "\n"
		}
	}
	return []byte(body)
}

func Render(rawBytes []byte) ([]byte, error) {
	result, err := RenderYaml(rawBytes)
	if err != nil {
		return nil, err
	}
	return Sanitizer.SanitizeBytes(result), nil
}
