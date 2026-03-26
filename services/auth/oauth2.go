// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/httpauth"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/services/actions"
	"code.gitea.io/gitea/services/oauth2_provider"
)

// Ensure the struct implements the interface.
var (
	_ Method = &OAuth2{}
)

// GetOAuthAccessTokenScopeAndUserID returns access token scope and user id
func GetOAuthAccessTokenScopeAndUserID(ctx context.Context, accessToken string) (auth_model.AccessTokenScope, int64) {
	var accessTokenScope auth_model.AccessTokenScope
	if !setting.OAuth2.Enabled {
		return accessTokenScope, 0
	}

	// JWT tokens require a ".", if the token isn't like that, return early
	if !strings.Contains(accessToken, ".") {
		return accessTokenScope, 0
	}

	token, err := oauth2_provider.ParseToken(accessToken, oauth2_provider.DefaultSigningKey)
	if err != nil {
		log.Trace("oauth2.ParseToken: %v", err)
		return accessTokenScope, 0
	}
	var grant *auth_model.OAuth2Grant
	if grant, err = auth_model.GetOAuth2GrantByID(ctx, token.GrantID); err != nil || grant == nil {
		return accessTokenScope, 0
	}
	if token.Kind != oauth2_provider.KindAccessToken {
		return accessTokenScope, 0
	}
	if token.ExpiresAt.Before(time.Now()) || token.IssuedAt.After(time.Now()) {
		return accessTokenScope, 0
	}
	accessTokenScope = oauth2_provider.GrantAdditionalScopes(grant.Scope)
	return accessTokenScope, grant.UserID
}

// CheckTaskIsRunning verifies that the TaskID corresponds to a running task
func CheckTaskIsRunning(ctx context.Context, taskID int64) bool {
	// Verify the task exists
	task, err := actions_model.GetTaskByID(ctx, taskID)
	if err != nil {
		return false
	}

	// Verify that it's running
	return task.Status == actions_model.StatusRunning
}

// OAuth2 implements the Auth interface and authenticates requests
// (API requests only) by looking for an OAuth token in query parameters or the
// "Authorization" header.
type OAuth2 struct{}

// Name represents the name of auth method
func (o *OAuth2) Name() string {
	return "oauth2"
}

// parseToken returns the token from request, and a boolean value
// representing whether the token exists or not
func parseToken(req *http.Request) (string, bool) {
	_ = req.ParseForm()
	if !setting.DisableQueryAuthToken {
		// Check token.
		if token := req.Form.Get("token"); token != "" {
			return token, true
		}
		// Check access token.
		if token := req.Form.Get("access_token"); token != "" {
			return token, true
		}
	} else if req.Form.Get("token") != "" || req.Form.Get("access_token") != "" {
		log.Warn("API token sent in query string but DISABLE_QUERY_AUTH_TOKEN=true")
	}

	// check header token
	if auHead := req.Header.Get("Authorization"); auHead != "" {
		parsed, ok := httpauth.ParseAuthorizationHeader(auHead)
		if ok && parsed.BearerToken != nil {
			return parsed.BearerToken.Token, true
		}
	}
	return "", false
}

// userIDFromToken returns the user id corresponding to the OAuth token.
// It will set 'IsApiToken' to true if the token is an API token and
// set 'ApiTokenScope' to the scope of the access token
func (o *OAuth2) userIDFromToken(ctx context.Context, tokenSHA string, store DataStore) int64 {
	// Let's see if token is valid.
	if strings.Contains(tokenSHA, ".") {
		// First attempt to decode an actions JWT, returning the actions user
		if taskID, err := actions.TokenToTaskID(tokenSHA); err == nil {
			if CheckTaskIsRunning(ctx, taskID) {
				store.GetData()["IsActionsToken"] = true
				store.GetData()["ActionsTaskID"] = taskID
				return user_model.ActionsUserID
			}
		}

		// Otherwise, check if this is an OAuth access token
		accessTokenScope, uid := GetOAuthAccessTokenScopeAndUserID(ctx, tokenSHA)
		if uid != 0 {
			store.GetData()["IsApiToken"] = true
			store.GetData()["ApiTokenScope"] = accessTokenScope
		}
		return uid
	}
	t, err := auth_model.GetAccessTokenBySHA(ctx, tokenSHA)
	if err != nil {
		if auth_model.IsErrAccessTokenNotExist(err) {
			log.Warn("OAuth2 Debug: GetAccessTokenBySHA token NOT FOUND in DB: token=%s err=%v", tokenSHA, err)
			// check task token
			task, err := actions_model.GetRunningTaskByToken(ctx, tokenSHA)
			if err == nil && task != nil {
				log.Trace("Basic Authorization: Valid AccessToken for task[%d]", task.ID)

				store.GetData()["IsActionsToken"] = true
				store.GetData()["ActionsTaskID"] = task.ID

				return user_model.ActionsUserID
			}
		} else if !auth_model.IsErrAccessTokenNotExist(err) && !auth_model.IsErrAccessTokenEmpty(err) {
			log.Error("OAuth2 Debug: GetAccessTokenBySHA DB ERROR: token=%s err=%v", tokenSHA, err)
		} else {
			log.Warn("OAuth2 Debug: GetAccessTokenBySHA empty/other: token=%s err=%v", tokenSHA, err)
		}
		return 0
	}
	log.Warn("OAuth2 Debug: GetAccessTokenBySHA SUCCESS: token=%s uid=%d tokenID=%d name=%s", tokenSHA, t.UID, t.ID, t.Name)
	t.UpdatedUnix = timeutil.TimeStampNow()
	if err = auth_model.UpdateAccessToken(ctx, t); err != nil {
		log.Error("UpdateAccessToken: %v", err)
	}
	store.GetData()["IsApiToken"] = true
	store.GetData()["ApiTokenScope"] = t.Scope
	return t.UID
}

// isDebugAuthPath returns true for API paths we want verbose auth logging on.
func isDebugAuthPath(req *http.Request) bool {
	p := req.URL.Path
	return p == "/api/v1/user" || p == "/api/v1/version"
}

// Verify extracts the user ID from the OAuth token in the query parameters
// or the "Authorization" header and returns the corresponding user object for that ID.
// If verification is successful returns an existing user object.
// Returns nil if verification fails.
func (o *OAuth2) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) (*user_model.User, error) {
	// These paths are not API paths, but we still want to check for tokens because they maybe in the API returned URLs
	detector := newAuthPathDetector(req)
	if !detector.isAPIPath() && !detector.isAttachmentDownload() && !detector.isAuthenticatedTokenRequest() &&
		!detector.isGitRawOrAttachPath() && !detector.isArchivePath() {
		return nil, nil
	}

	debugPath := isDebugAuthPath(req)

	token, ok := parseToken(req)
	if !ok {
		if debugPath {
			log.Warn("OAuth2 Debug: %s %s — NO TOKEN in request. Authorization header: [%s]",
				req.Method, req.URL.RequestURI(), req.Header.Get("Authorization"))
		}
		return nil, nil
	}

	if debugPath {
		log.Warn("OAuth2 Debug: %s %s — token from request: [%s] (len=%d) Authorization header: [%s]",
			req.Method, req.URL.RequestURI(), token, len(token), req.Header.Get("Authorization"))
	}

	id := o.userIDFromToken(req.Context(), token, store)

	if id <= 0 && id != -2 { // -2 means actions, so we need to allow it.
		if debugPath {
			log.Warn("OAuth2 Debug: %s %s — token lookup FAILED, userID=%d, token=[%s]",
				req.Method, req.URL.RequestURI(), id, token)
		}
		// Token is invalid or expired — treat as unauthenticated rather than
		// returning an error. This allows public endpoints (e.g. /api/v1/version)
		// to succeed even when the request carries a stale token.
		// Protected endpoints enforce auth via reqToken() middleware.
		return nil, nil
	}
	log.Trace("OAuth2 Authorization: Found token for user[%d]", id)

	user, err := user_model.GetPossibleUserByID(req.Context(), id)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			log.Error("GetUserByName: %v", err)
			return nil, err
		}
		if debugPath {
			log.Warn("OAuth2 Debug: %s %s — user NOT FOUND for id=%d, token=[%s]",
				req.Method, req.URL.RequestURI(), id, token)
		}
		// User was deleted but token still exists — treat as unauthenticated
		return nil, nil
	}

	if debugPath {
		log.Warn("OAuth2 Debug: %s %s — SUCCESS user=%s (id=%d) token=[%s]",
			req.Method, req.URL.RequestURI(), user.Name, user.ID, token)
	}
	log.Trace("OAuth2 Authorization: Logged in user %-v", user)
	return user, nil
}
