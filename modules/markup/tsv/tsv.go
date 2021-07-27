// Copyright 2021 The unfoldingWord Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bytes"
	"encoding/csv"
	"html"
	"io"
	"regexp"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
)

var breakRegexp = regexp.MustCompile(`<br\/*>`)

func init() {
	markup.RegisterParser(Parser{})

}

// Parser implements markup.Parser for tsv
type Parser struct {
}

// Name implements markup.Parser
func (Parser) Name() string {
	return "tsv"
}

// Extensions implements markup.Parser
func (Parser) Extensions() []string {
	return []string{".tsv"}
}

// Render implements markup.Parser
func (p Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	rd := csv.NewReader(bytes.NewReader(rawBytes))
	rd.Comma = '\t'
	var tmpBlock bytes.Buffer
	tmpBlock.WriteString(`<table class="table tsv">`)
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
		tmpBlock.WriteString("<tr>")
		for colID, field := range fields {
			if rowId == 0 && strings.HasSuffix(strings.ToLower(field), "note") {
				noteID = colID
			}
			if rowID > 0 && colID == noteID {
				tmpBlock.WriteString(`<td class="note">`)
				tmpBlock.WriteString(string(markdown.Render([]byte(breakRegexp.ReplaceAllString(field, "\n")), urlPrefix, metas)))
			} else {
				tmpBlock.WriteString("<td>")
				tmpBlock.WriteString(html.EscapeString(field))
			}
			tmpBlock.WriteString("</td>")
		}
		tmpBlock.WriteString("</tr>")
		rowID += 1
	}
	tmpBlock.WriteString("</table>")

	return tmpBlock.Bytes()
}
