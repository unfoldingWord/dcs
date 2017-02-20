package hashtag_test

import (
	"testing"

	"code.gitea.io/gitea/models"
	. "code.gitea.io/gitea/modules/hashtag"
	"code.gitea.io/gitea/modules/markdown"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestHashtagUBN(t *testing.T) {
	user := &models.User{ID: 1, LowerName: "username"}
	setting.AppSubURL = "http://example.com"

	repo := &models.Repository{
		ID:        1,
		LowerName: "en-ubn-act",
		Owner:     user,
	}

	assert.Equal(t,
		`<a href="http://example.com/username/en-ubn/hashtags/testtag">#testtag</a>`,
		string(ConvertHashtagsToLinks(repo, []byte(`#testtag`))))

	markdownContent := []byte(
		`# Acts 1
#author-luke

A hashtag in this line such as #test should not be rendered, but the
following should be rendered as links except for #v12 which should not be a link:
#v12
#kingdomofgod
#da-god
`)
	htmlContent := markdown.Render(markdownContent, "content/01.md", nil)
	convertedHashtags := ConvertHashtagsToLinks(repo, htmlContent)
	assert.Equal(t,
		`<h1>Acts 1</h1>

<p><a href="http://example.com/username/en-ubn/hashtags/author-luke">#author-luke</a></p>

<p>A hashtag in this line such as #test should not be rendered, but the
following should be rendered as links except for #v12 which should not be a link:
#v12
<a href="http://example.com/username/en-ubn/hashtags/kingdomofgod">#kingdomofgod</a>
<a href="http://example.com/username/en-ubn/hashtags/da-god">#da-god</a></p>
`, string(convertedHashtags))
}

func TestHashtagNonUBN(t *testing.T) {
	user := &models.User{ID: 1, LowerName: "username"}
	setting.AppSubURL = "http://example.com"

	repo := &models.Repository{
		ID:        1,
		LowerName: "en-act",
		Owner:     user,
	}

	assert.Equal(t, `#testtag`, string(ConvertHashtagsToLinks(repo, []byte(`#testtag`))))

	markdownContent := []byte(
		`<h1>Acts 1</h1>

<p>#author-luke</p>

<p>No hashtags should be linked since this is not a ubn repo.
#v12
#kingdomofgod
#da-god</p>
`)
	htmlContent := markdown.Render(markdownContent, "content/01.md", nil)
	convertedHashtags := ConvertHashtagsToLinks(repo, htmlContent)
	assert.Equal(t,
		`<h1>Acts 1</h1>

<p>#author-luke</p>

<p>No hashtags should be linked since this is not a ubn repo.
#v12
#kingdomofgod
#da-god</p>
`, string(convertedHashtags))
}
