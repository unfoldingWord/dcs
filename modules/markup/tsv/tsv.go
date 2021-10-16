// Copyright 2021 The unfoldingWord Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"html"
	"io"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
)

var breakRegexp = regexp.MustCompile(`<br\/*>`)

func init() {
	markup.RegisterRenderer(Renderer{})
}

// Renderer implements markup.Renderer for csv files
type Renderer struct {
}

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

func writeField(w io.Writer, element, class, field string) error {
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
	if _, err := io.WriteString(w, html.EscapeString(field)); err != nil {
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
	var tmpBlock = bufio.NewWriter(output)

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

	if _, err := tmpBlock.WriteString(`<table class="data-table">`); err != nil {
		return err
	}
	rowID := 0
	noteID := -1
	for {
		fields, err := rd.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if _, err := tmpBlock.WriteString("<tr>"); err != nil {
			return err
		}
		element := "td"
		if rowID == 0 {
			element = "th"
		}
		if err := writeField(tmpBlock, element, "line-num", strconv.Itoa(rowID+1)); err != nil {
			return err
		}
		for colID, field := range fields {
			if rowID == 0 && strings.HasSuffix(strings.ToLower(field), "note") {
				noteID = colID
			}
			if rowID > 0 && colID == noteID {
				tmpBlock.WriteString(`<td class="note">`)
				renderedString, err := markdown.RenderString(&markup.RenderContext{
					URLPrefix: ctx.URLPrefix,
					Metas:     ctx.Metas,
					IsWiki:    true,
				}, breakRegexp.ReplaceAllString(field, "\n"))
				if err != nil {
					tmpBlock.WriteString(field)
				}
				tmpBlock.WriteString(renderedString)
			} else {
				tmpBlock.WriteString("<td>")
				tmpBlock.WriteString(html.EscapeString(field))
			}

			if err := writeField(tmpBlock, element, "", field); err != nil {
				return err
			}
		}
		if _, err := tmpBlock.WriteString("</tr>"); err != nil {
			return err
		}

		rowID++
	}
	if _, err = tmpBlock.WriteString("</table>"); err != nil {
		return err
	}
	return tmpBlock.Flush()
}
