// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/dcs"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// ToRepo converts a Repository to api.Repository
func ToRepo(repo *repo_model.Repository, mode perm.AccessMode) *api.Repository {
	return innerToRepo(repo, mode, false)
}

func innerToRepo(repo *repo_model.Repository, mode perm.AccessMode, isParent bool) *api.Repository {
	var parent *api.Repository

	cloneLink := repo.CloneLink()
	permission := &api.Permission{
		Admin: mode >= perm.AccessModeAdmin,
		Push:  mode >= perm.AccessModeWrite,
		Pull:  mode >= perm.AccessModeRead,
	}
	if !isParent {
		err := repo.GetBaseRepo()
		if err != nil {
			return nil
		}
		if repo.BaseRepo != nil {
			parent = innerToRepo(repo.BaseRepo, mode, true)
		}
	}

	// check enabled/disabled units
	hasIssues := false
	var externalTracker *api.ExternalTracker
	var internalTracker *api.InternalTracker
	if unit, err := repo.GetUnit(unit_model.TypeIssues); err == nil {
		config := unit.IssuesConfig()
		hasIssues = true
		internalTracker = &api.InternalTracker{
			EnableTimeTracker:                config.EnableTimetracker,
			AllowOnlyContributorsToTrackTime: config.AllowOnlyContributorsToTrackTime,
			EnableIssueDependencies:          config.EnableDependencies,
		}
	} else if unit, err := repo.GetUnit(unit_model.TypeExternalTracker); err == nil {
		config := unit.ExternalTrackerConfig()
		hasIssues = true
		externalTracker = &api.ExternalTracker{
			ExternalTrackerURL:    config.ExternalTrackerURL,
			ExternalTrackerFormat: config.ExternalTrackerFormat,
			ExternalTrackerStyle:  config.ExternalTrackerStyle,
		}
	}
	hasWiki := false
	var externalWiki *api.ExternalWiki
	if _, err := repo.GetUnit(unit_model.TypeWiki); err == nil {
		hasWiki = true
	} else if unit, err := repo.GetUnit(unit_model.TypeExternalWiki); err == nil {
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
	defaultMergeStyle := repo_model.MergeStyleMerge
	if unit, err := repo.GetUnit(unit_model.TypePullRequests); err == nil {
		config := unit.PullRequestsConfig()
		hasPullRequests = true
		ignoreWhitespaceConflicts = config.IgnoreWhitespaceConflicts
		allowMerge = config.AllowMerge
		allowRebase = config.AllowRebase
		allowRebaseMerge = config.AllowRebaseMerge
		allowSquash = config.AllowSquash
		defaultMergeStyle = config.GetDefaultMergeStyle()
	}
	hasProjects := false
	if _, err := repo.GetUnit(unit_model.TypeProjects); err == nil {
		hasProjects = true
	}

	if err := repo.GetOwner(db.DefaultContext); err != nil {
		return nil
	}

	numReleases, _ := models.GetReleaseCountByRepoID(repo.ID, models.FindReleasesOptions{IncludeDrafts: false, IncludeTags: false})

	/*** DCS Customizations ***/

	// TODO: Load in Repository's LoadAttributes() function and save to repo.Metadata
	metadata, err := models.GetDoor43MetadataByRepoIDAndReleaseID(repo.ID, 0)
	if err != nil && !models.IsErrDoor43MetadataNotExist(err) {
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
	var alignmentCounts map[string]interface{}
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
			alignmentCounts = val.(map[string]interface{})
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
		repo.Mirror, err = repo_model.GetMirrorByRepoID(db.DefaultContext, repo.ID)
		if err == nil {
			mirrorInterval = repo.Mirror.Interval.String()
			mirrorUpdated = repo.Mirror.UpdatedUnix.AsTime()
		}
	}

	var transfer *api.RepoTransfer
	if repo.Status == repo_model.RepositoryPendingTransfer {
		t, err := models.GetPendingRepositoryTransfer(repo)
		if err != nil && !models.IsErrNoPendingTransfer(err) {
			log.Warn("GetPendingRepositoryTransfer: %v", err)
		} else {
			if err := t.LoadAttributes(); err != nil {
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
		ID:                        repo.ID,
		Owner:                     ToUserWithAccessMode(repo.Owner, mode),
		Name:                      repo.Name,
		FullName:                  repo.FullName(),
		Description:               repo.Description,
		Private:                   repo.IsPrivate,
		Template:                  repo.IsTemplate,
		Empty:                     repo.IsEmpty,
		Archived:                  repo.IsArchived,
		Size:                      int(repo.Size / 1024),
		Fork:                      repo.IsFork,
		Parent:                    parent,
		Mirror:                    repo.IsMirror,
		HTMLURL:                   repo.HTMLURL(),
		SSHURL:                    cloneLink.SSH,
		CloneURL:                  cloneLink.HTTPS,
		OriginalURL:               repo.SanitizedOriginalURL(),
		Website:                   repo.Website,
		LanguagesURL:              repoAPIURL + "/languages",
		Stars:                     repo.NumStars,
		Forks:                     repo.NumForks,
		Watchers:                  repo.NumWatches,
		OpenIssues:                repo.NumOpenIssues,
		OpenPulls:                 repo.NumOpenPulls,
		Releases:                  int(numReleases),
		DefaultBranch:             repo.DefaultBranch,
		Created:                   repo.CreatedUnix.AsTime(),
		Updated:                   repo.UpdatedUnix.AsTime(),
		Permissions:               permission,
		HasIssues:                 hasIssues,
		ExternalTracker:           externalTracker,
		InternalTracker:           internalTracker,
		HasWiki:                   hasWiki,
		HasProjects:               hasProjects,
		ExternalWiki:              externalWiki,
		HasPullRequests:           hasPullRequests,
		IgnoreWhitespaceConflicts: ignoreWhitespaceConflicts,
		AllowMerge:                allowMerge,
		AllowRebase:               allowRebase,
		AllowRebaseMerge:          allowRebaseMerge,
		AllowSquash:               allowSquash,
		DefaultMergeStyle:         string(defaultMergeStyle),
		AvatarURL:                 repo.AvatarLink(),
		Language:                  language,
		LanguageTitle:             languageTitle,
		LanguageDir:               languageDir,
		LanguageIsGL:              languageIsGL,
		Title:                     title,
		Subject:                   subject,
		Books:                     books,
		AlignmentCounts:           alignmentCounts,
		CheckingLevel:             checkingLevel,
		Internal:                  !repo.IsPrivate && repo.Owner.Visibility == api.VisibleTypePrivate,
		MirrorInterval:            mirrorInterval,
		MirrorUpdated:             mirrorUpdated,
		RepoTransfer:              transfer,
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
