// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package yaml

import (
	"fmt"
	"strings"
	"reflect"

	"gopkg.in/yaml.v2"
)

func renderHtmlTable(m map[interface{}]interface{}) []byte {
	var thead, tbody string
	for k, v := range m {
		thead += fmt.Sprintf("<th>%v</th>", k)
		rt := reflect.TypeOf(v)
		if rt != nil {
			switch rt.Kind() {
			case reflect.Map:
				tbody += fmt.Sprintf("<td>%s</td>", renderHtmlTable(v.(map[interface{}]interface{})))
			default:
				tbody += fmt.Sprintf("<td>%v</td>", v)
			}
		} else {
			tbody += fmt.Sprintf("<td>%v</td>", v)
		}
	}

	table := ""
	if len(thead) > 0 {
		table = fmt.Sprintf(`<table data="yaml-metadata"><thead><tr>%s</tr></thead><tbody><tr>%s</tr></table>`, thead, tbody)
	}
	return []byte(table)
}

func RenderYamlHtmlTable(data []byte) []byte {
	m := make(map[interface{}]interface{})

	if err := yaml.Unmarshal(data, &m); err != nil {
		return []byte("")
	}

	return renderHtmlTable(m)
}

func StripYamlFromText(data []byte) []byte {
	m := make(map[interface{}]interface{})
	if err := yaml.Unmarshal(data, &m); err != nil {
		return data
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
			body += line+"\n"
		}
	}
	return []byte(body)
}

// RenderString renders any YAML section (top of file, denoted by --- on first line and end of YAML)
// into an HTML table and appends ready of the text after
func Render(rawBytes []byte) []byte {
	htmlTable := RenderYamlHtmlTable(rawBytes)
	body := StripYamlFromText(rawBytes)
	return append(htmlTable, body...)
}

// Renders the YAML and text as a string
func RenderString(rawBytes []byte) string {
	return string(Render(rawBytes))
}

