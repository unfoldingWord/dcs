// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth/oauth2"

	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
)

func createAndParseToken(t *testing.T, grant *models.OAuth2Grant) *models.OIDCToken {
	signingKey, err := oauth2.CreateJWTSingingKey("HS256", make([]byte, 32))
	assert.NoError(t, err)
	assert.NotNil(t, signingKey)
	oauth2.DefaultSigningKey = signingKey

	response, terr := newAccessTokenResponse(grant, signingKey)
	assert.Nil(t, terr)
	assert.NotNil(t, response)

	parsedToken, err := jwt.ParseWithClaims(response.IDToken, &models.OIDCToken{}, func(token *jwt.Token) (interface{}, error) {
		assert.NotNil(t, token.Method)
		assert.Equal(t, signingKey.SigningMethod().Alg(), token.Method.Alg())
		return signingKey.VerifyKey(), nil
	})
	assert.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	oidcToken, ok := parsedToken.Claims.(*models.OIDCToken)
	assert.True(t, ok)
	assert.NotNil(t, oidcToken)

	return oidcToken
}

func TestNewAccessTokenResponse_OIDCToken(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	grants, err := models.GetOAuth2GrantsByUserID(3)
	assert.NoError(t, err)
	assert.Len(t, grants, 1)

	// Scopes: openid
	oidcToken := createAndParseToken(t, grants[0])
	assert.Empty(t, oidcToken.Name)
	assert.Empty(t, oidcToken.PreferredUsername)
	assert.Empty(t, oidcToken.Profile)
	assert.Empty(t, oidcToken.Picture)
	assert.Empty(t, oidcToken.Website)
	assert.Empty(t, oidcToken.UpdatedAt)
	assert.Empty(t, oidcToken.Email)
	assert.False(t, oidcToken.EmailVerified)

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 5}).(*models.User)
	grants, err = models.GetOAuth2GrantsByUserID(user.ID)
	assert.NoError(t, err)
	assert.Len(t, grants, 1)

	// Scopes: openid profile email
	oidcToken = createAndParseToken(t, grants[0])
	assert.Equal(t, user.FullName, oidcToken.Name)
	assert.Equal(t, user.Name, oidcToken.PreferredUsername)
	assert.Equal(t, user.HTMLURL(), oidcToken.Profile)
	assert.Equal(t, user.AvatarLink(), oidcToken.Picture)
	assert.Equal(t, user.Website, oidcToken.Website)
	assert.Equal(t, user.UpdatedUnix, oidcToken.UpdatedAt)
	assert.Equal(t, user.Email, oidcToken.Email)
	assert.Equal(t, user.IsActive, oidcToken.EmailVerified)
}
