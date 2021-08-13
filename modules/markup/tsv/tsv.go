// Copyright 2021 The unfoldingWord Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bufio"
	"bytes"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"regexp"
	"strconv"

	"code.gitea.io/gitea/modules/csv"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
)

func init() {
	markup.RegisterRenderer(Renderer{})
}

// Renderer implements markup.Renderer for csv files
type Renderer struct {
}

// Name implements markup.Renderer
func (Renderer) Name() string {
	return "tsv"
}

// NeedPostProcess implements markup.Renderer
func (Renderer) NeedPostProcess() bool { return false }

// Extensions implements markup.Renderer
func (Renderer) Extensions() []string {
	return []string{".tsv"}
}

// SanitizerRules implements markup.Renderer
func (Renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return []setting.MarkupSanitizerRule{
		{Element: "table", AllowAttr: "class", Regexp: regexp.MustCompile(`data-table`)},
		{Element: "th", AllowAttr: "class", Regexp: regexp.MustCompile(`line-num`)},
		{Element: "td", AllowAttr: "class", Regexp: regexp.MustCompile(`markdown-to-html`)},
	}
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
	var tmpBlock = bufio.NewWriter(output)

	// FIXME: don't read all to memory
	rawBytes, err := ioutil.ReadAll(input)
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

	rd, err := csv.CreateReaderAndGuessDelimiter(bytes.NewReader(rawBytes))
	if err != nil {
		return err
	}
	rd.Comma = '\t' // This is a .tsv file so assume \t is delimiter
	rd.LazyQuotes = true
	rd.TrimLeadingSpace = false

	if _, err := tmpBlock.WriteString(`<table class="data-table tsv">`); err != nil {
		return err
	}
	row := 1
	numFields := -1
	newlineRegexp := regexp.MustCompile(`(<br\/*>|\\n)`)
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
		}
		if err := writeField(tmpBlock, element, "line-num", strconv.Itoa(row), true); err != nil {
			return err
		}
		for _, field := range fields {
			if row > 1 {
				if html, err := markdown.RenderString(&markup.RenderContext{URLPrefix: ctx.URLPrefix, Metas: ctx.Metas},
					newlineRegexp.ReplaceAllString(field, "\n")); err != nil {
					return err
				} else if err := writeField(tmpBlock, element, "markdown-to-html", html, false); err != nil {
					return err
				}
			} else {
				if err := writeField(tmpBlock, element, "", field, true); err != nil {
					return err
				}
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
