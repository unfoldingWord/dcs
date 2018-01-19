package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestGoogleAnalytics(t *testing.T) {
	// test that the google analytics javascript is included in HTML
	prepareTestEnv(t)
	setting.Google.GATrackingID = "UA-012345-6"

	req := NewRequest(t, "GET", "/")
	resp := MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	scripts := htmlDoc.doc.Find("script").Text()
	assert.Contains(t, scripts, setting.Google.GATrackingID)
}
