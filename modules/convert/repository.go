// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/dcs"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// ToRepo converts a Repository to api.Repository
func ToRepo(ctx context.Context, repo *repo_model.Repository, mode perm.AccessMode) *api.Repository {
	return innerToRepo(ctx, repo, mode, false)
}

func innerToRepo(ctx context.Context, repo *repo_model.Repository, mode perm.AccessMode, isParent bool) *api.Repository {
	var parent *api.Repository

	cloneLink := repo.CloneLink()
	permission := &api.Permission{
		Admin: mode >= perm.AccessModeAdmin,
		Push:  mode >= perm.AccessModeWrite,
		Pull:  mode >= perm.AccessModeRead,
	}
	if !isParent {
		err := repo.GetBaseRepo(ctx)
		if err != nil {
			return nil
		}
		if repo.BaseRepo != nil {
			parent = innerToRepo(ctx, repo.BaseRepo, mode, true)
		}
	}

	// check enabled/disabled units
	hasIssues := false
	var externalTracker *api.ExternalTracker
	var internalTracker *api.InternalTracker
	if unit, err := repo.GetUnit(ctx, unit_model.TypeIssues); err == nil {
		config := unit.IssuesConfig()
		hasIssues = true
		internalTracker = &api.InternalTracker{
			EnableTimeTracker:                config.EnableTimetracker,
			AllowOnlyContributorsToTrackTime: config.AllowOnlyContributorsToTrackTime,
			EnableIssueDependencies:          config.EnableDependencies,
		}
	} else if unit, err := repo.GetUnit(ctx, unit_model.TypeExternalTracker); err == nil {
		config := unit.ExternalTrackerConfig()
		hasIssues = true
		externalTracker = &api.ExternalTracker{
			ExternalTrackerURL:           config.ExternalTrackerURL,
			ExternalTrackerFormat:        config.ExternalTrackerFormat,
			ExternalTrackerStyle:         config.ExternalTrackerStyle,
			ExternalTrackerRegexpPattern: config.ExternalTrackerRegexpPattern,
		}
	}
	hasWiki := false
	var externalWiki *api.ExternalWiki
	if _, err := repo.GetUnit(ctx, unit_model.TypeWiki); err == nil {
		hasWiki = true
	} else if unit, err := repo.GetUnit(ctx, unit_model.TypeExternalWiki); err == nil {
		hasWiki = true
		config := unit.ExternalWikiConfig()
		externalWiki = &api.ExternalWiki{
			ExternalWikiURL: config.ExternalWikiURL,
		}
	}
	hasPullRequests := false
	ignoreWhitespaceConflicts := false
	allowMerge := false
	allowRebase := false
	allowRebaseMerge := false
	allowSquash := false
	allowRebaseUpdate := false
	defaultDeleteBranchAfterMerge := false
	defaultMergeStyle := repo_model.MergeStyleMerge
	if unit, err := repo.GetUnit(ctx, unit_model.TypePullRequests); err == nil {
		config := unit.PullRequestsConfig()
		hasPullRequests = true
		ignoreWhitespaceConflicts = config.IgnoreWhitespaceConflicts
		allowMerge = config.AllowMerge
		allowRebase = config.AllowRebase
		allowRebaseMerge = config.AllowRebaseMerge
		allowSquash = config.AllowSquash
		allowRebaseUpdate = config.AllowRebaseUpdate
		defaultDeleteBranchAfterMerge = config.DefaultDeleteBranchAfterMerge
		defaultMergeStyle = config.GetDefaultMergeStyle()
	}
	hasProjects := false
	if _, err := repo.GetUnit(ctx, unit_model.TypeProjects); err == nil {
		hasProjects = true
	}

	if err := repo.GetOwner(ctx); err != nil {
		return nil
	}

	numReleases, _ := repo_model.GetReleaseCountByRepoID(ctx, repo.ID, repo_model.FindReleasesOptions{IncludeDrafts: false, IncludeTags: false})

	/*** DCS Customizations ***/

	// TODO: Load in Repository's LoadAttributes() function and save to repo.Metadata
	metadata, err := repo_model.GetDoor43MetadataByRepoIDAndReleaseID(repo.ID, 0)
	if err != nil && !repo_model.IsErrDoor43MetadataNotExist(err) {
		log.Error("GetDoor43MetadataByRepoIDAndReleaseID: %v", err)
	}
	// if metadata == nil {
	// 	metadata, err = repo.GetLatestPreProdCatalogMetadata()
	// 	if err != nil {
	// 		log.Error("GetLatestPreProdCatalogMetadata: %v", err)
	// 	}
	// }

	var language, languageTitle, languageDir, title, subject, checkingLevel string
	var languageIsGL bool
	var books []string
	var alignmentCounts map[string]int64
	if metadata != nil {
		title = (*metadata.Metadata)["dublin_core"].(map[string]interface{})["title"].(string)
		subject = (*metadata.Metadata)["dublin_core"].(map[string]interface{})["subject"].(string)

		if val, ok := (*metadata.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["identifier"].(string); ok {
			language = val
		}

		if val, ok := (*metadata.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["direction"].(string); ok {
			languageDir = val
		} else if language != "" {
			languageDir = dcs.GetLanguageDirection(language)
		}

		if val, ok := (*metadata.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["title"].(string); ok {
			languageTitle = val
		} else if language != "" {
			languageTitle = dcs.GetLanguageTitle(language)
		}

		if val, ok := (*metadata.Metadata)["dublin_core"].(map[string]interface{})["language"].(map[string]interface{})["is_gl"].(bool); ok {
			languageIsGL = val
		} else {
			languageIsGL = dcs.LanguageIsGL(language)
		}

		if val, ok := (*metadata.Metadata)["books"].([]string); ok {
			books = val
		} else {
			books = metadata.GetBooks()
		}

		if val, ok := (*metadata.Metadata)["alignment_counts"]; ok {
			// Marshal/Unmarshal to let Unmarshaliing convert interface{} to map[string]int64
			if byteData, err := json.Marshal(val); err == nil {
				if err := json.Unmarshal(byteData, &alignmentCounts); err != nil {
					log.Error("Unable to Unmarshal alignment_counts: %v\n", val)
				}
			}
		}

		checkingLevel = fmt.Sprintf("%v", (*metadata.Metadata)["checking"].(map[string]interface{})["checking_level"])
	} else {
		subject = dcs.GetSubjectFromRepoName(repo.LowerName)
		language = dcs.GetLanguageFromRepoName(repo.LowerName)
		languageTitle = dcs.GetLanguageTitle(language)
		languageDir = dcs.GetLanguageDirection(language)
		languageIsGL = dcs.LanguageIsGL(language)
	}
	/*** END DCS Customizations ***/

	mirrorInterval := ""
	var mirrorUpdated time.Time
	if repo.IsMirror {
		var err error
		repo.Mirror, err = repo_model.GetMirrorByRepoID(ctx, repo.ID)
		if err == nil {
			mirrorInterval = repo.Mirror.Interval.String()
			mirrorUpdated = repo.Mirror.UpdatedUnix.AsTime()
		}
	}

	var transfer *api.RepoTransfer
	if repo.Status == repo_model.RepositoryPendingTransfer {
		t, err := models.GetPendingRepositoryTransfer(ctx, repo)
		if err != nil && !models.IsErrNoPendingTransfer(err) {
			log.Warn("GetPendingRepositoryTransfer: %v", err)
		} else {
			if err := t.LoadAttributes(ctx); err != nil {
				log.Warn("LoadAttributes of RepoTransfer: %v", err)
			} else {
				transfer = ToRepoTransfer(t)
			}
		}
	}

	/*** DCS Customizations - Commented out for Resource Language instead of programming language ***/
	// var language string
	// if repo.PrimaryLanguage != nil {
	// 	language = repo.PrimaryLanguage.Language
	// }
	/*** END DCS Customizaitons ***/

	repoAPIURL := repo.APIURL()

	return &api.Repository{
		ID:                            repo.ID,
		Owner:                         ToUserWithAccessMode(repo.Owner, mode),
		Name:                          repo.Name,
		FullName:                      repo.FullName(),
		Description:                   repo.Description,
		Private:                       repo.IsPrivate,
		Template:                      repo.IsTemplate,
		Empty:                         repo.IsEmpty,
		Archived:                      repo.IsArchived,
		Size:                          int(repo.Size / 1024),
		Fork:                          repo.IsFork,
		Parent:                        parent,
		Mirror:                        repo.IsMirror,
		HTMLURL:                       repo.HTMLURL(),
		SSHURL:                        cloneLink.SSH,
		CloneURL:                      cloneLink.HTTPS,
		OriginalURL:                   repo.SanitizedOriginalURL(),
		Website:                       repo.Website,
		LanguagesURL:                  repoAPIURL + "/languages",
		Stars:                         repo.NumStars,
		Forks:                         repo.NumForks,
		Watchers:                      repo.NumWatches,
		OpenIssues:                    repo.NumOpenIssues,
		OpenPulls:                     repo.NumOpenPulls,
		Releases:                      int(numReleases),
		DefaultBranch:                 repo.DefaultBranch,
		Created:                       repo.CreatedUnix.AsTime(),
		Updated:                       repo.UpdatedUnix.AsTime(),
		Permissions:                   permission,
		HasIssues:                     hasIssues,
		ExternalTracker:               externalTracker,
		InternalTracker:               internalTracker,
		HasWiki:                       hasWiki,
		HasProjects:                   hasProjects,
		ExternalWiki:                  externalWiki,
		HasPullRequests:               hasPullRequests,
		IgnoreWhitespaceConflicts:     ignoreWhitespaceConflicts,
		AllowMerge:                    allowMerge,
		AllowRebase:                   allowRebase,
		AllowRebaseMerge:              allowRebaseMerge,
		AllowSquash:                   allowSquash,
		AllowRebaseUpdate:             allowRebaseUpdate,
		DefaultDeleteBranchAfterMerge: defaultDeleteBranchAfterMerge,
		DefaultMergeStyle:             string(defaultMergeStyle),
		AvatarURL:                     repo.AvatarLink(),
		Internal:                      !repo.IsPrivate && repo.Owner.Visibility == api.VisibleTypePrivate,
		MirrorInterval:                mirrorInterval,
		MirrorUpdated:                 mirrorUpdated,
		RepoTransfer:                  transfer,
		Language:                      language,        // DCS Customization
		LanguageTitle:                 languageTitle,   // DCS Customization
		LanguageDir:                   languageDir,     // DCS Customization
		LanguageIsGL:                  languageIsGL,    // DCS Customization
		Title:                         title,           // DCS Customization
		Subject:                       subject,         // DCS Customization
		Books:                         books,           // DCS Customization
		AlignmentCounts:               alignmentCounts, // DCS Customization
		CheckingLevel:                 checkingLevel,   // DCS Customization
	}
}

// ToRepoTransfer convert a models.RepoTransfer to a structs.RepeTransfer
func ToRepoTransfer(t *models.RepoTransfer) *api.RepoTransfer {
	teams, _ := ToTeams(t.Teams, false)

	return &api.RepoTransfer{
		Doer:      ToUser(t.Doer, nil),
		Recipient: ToUser(t.Recipient, nil),
		Teams:     teams,
	}
}
