// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"regexp"
	"strconv"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"

	"github.com/yuin/goldmark"
)

func init() {
	markup.RegisterRenderer(Renderer{})
}

// Renderer implements markup.Renderer for csv files
type Renderer struct{}

// Name implements markup.Parser
func (Renderer) Name() string {
	return "tsv"
}

// Extensions implements markup.Parser
func (Renderer) Extensions() []string {
	return []string{".tsv"}
}

// NeedPostProcess implements markup.Renderer
func (Renderer) NeedPostProcess() bool { return false }

// SanitizerRules implements markup.Renderer
func (Renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return []setting.MarkupSanitizerRule{
		{Element: "table", AllowAttr: "class", Regexp: regexp.MustCompile(`data-table`)},
		{Element: "th", AllowAttr: "class", Regexp: regexp.MustCompile(`line-num`)},
		{Element: "td", AllowAttr: "class", Regexp: regexp.MustCompile(`line-num`)},
	}
}

// SanitizerDisabled disabled sanitize if return true
func (Renderer) SanitizerDisabled() bool {
	return false
}

func writeField(w io.Writer, element, class, field string, escapeString bool) error {
	if _, err := io.WriteString(w, "<"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, element); err != nil {
		return err
	}
	if len(class) > 0 {
		if _, err := io.WriteString(w, " class=\""); err != nil {
			return err
		}
		if _, err := io.WriteString(w, class); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\""); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, ">"); err != nil {
		return err
	}
	if escapeString {
		field = html.EscapeString(field)
	}
	if _, err := io.WriteString(w, field); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "</"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, element); err != nil {
		return err
	}
	_, err := io.WriteString(w, ">")
	return err
}

// Render implements markup.Renderer
func (Renderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	tmpBlock := bufio.NewWriter(output)

	// FIXME: don't read all to memory
	rawBytes, err := io.ReadAll(input)
	if err != nil {
		return err
	}

	if setting.UI.CSV.MaxFileSize != 0 && setting.UI.CSV.MaxFileSize < int64(len(rawBytes)) {
		if _, err := tmpBlock.WriteString("<pre>"); err != nil {
			return err
		}
		if _, err := tmpBlock.WriteString(html.EscapeString(string(rawBytes))); err != nil {
			return err
		}
		_, err = tmpBlock.WriteString("</pre>")
		return err
	}

	rd := csv.NewReader(bytes.NewReader(rawBytes))
	rd.Comma = '\t'

	if _, err := tmpBlock.WriteString(`<table class="data-table tsv">`); err != nil {
		return err
	}
	row := 1
	numFields := -1
	markdownField := regexp.MustCompile("(?i)(note|question|answer|response)")
	newlineRegexp := regexp.MustCompile(`(<br\/*>|\\n)`)
	rcFromBrackets := regexp.MustCompile(`\[\[(rc://[^\]]+)\]\]`)
	rcToBrackets := regexp.MustCompile(`START(rc://.+?)END`)
	var headers []string
	for {
		fields, fieldErr := rd.Read()
		if fieldErr == io.EOF {
			break
		}
		if err != nil {
			colspan := 1
			if numFields > 0 {
				colspan = numFields
			}
			if _, err := tmpBlock.WriteString(fmt.Sprintf(`<tr><td colspan="%d">%v</td></tr>`, colspan, err)); err != nil {
				return err
			}
			continue
		}
		if numFields < 0 {
			numFields = len(fields)
		}
		if _, err := tmpBlock.WriteString("<tr>"); err != nil {
			return err
		}
		element := "td"
		if row == 1 {
			element = "th"
			headers = fields
		}
		if err := writeField(tmpBlock, element, "line-num", strconv.Itoa(row), true); err != nil {
			return err
		}
		for i, field := range fields {
			if row > 1 {
				if len(headers) <= (i+1) && markdownField.MatchString(headers[i]) {
					md := newlineRegexp.ReplaceAllString(field, "\n")
					md = rcFromBrackets.ReplaceAllString(md, "START${1}END") // preserver rc links with double square brackets [[rc://...]] since that means something in markdown (short link)
					var buf bytes.Buffer
					if err := goldmark.Convert([]byte(md), &buf); err == nil {
						md = rcToBrackets.ReplaceAllString(buf.String(), "[[${1}]]") // restore double bracket rc links
						if err = writeField(tmpBlock, element, "", md, false); err == nil {
							continue
						}
					}
				}
			}
			// Did NOT render Markdown above, so going to make it a regular field
			if err := writeField(tmpBlock, element, "", field, true); err != nil {
				return err
			}
		}
		if _, err := tmpBlock.WriteString("</tr>"); err != nil {
			return err
		}

		row++
	}
	if _, err = tmpBlock.WriteString("</table>"); err != nil {
		return err
	}
	return tmpBlock.Flush()
}
