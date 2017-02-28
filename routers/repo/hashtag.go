package repo

import (
	"strings"
	"net/http"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

const (
	HASHTAGS    base.TplName = "repo/hashtag/list"
	HASHTAG     base.TplName = "repo/hashtag/page"
)

// Hashtags produces a page listing all hashtags for this language, with counts
func Hashtags(ctx *context.Context) {
	// get the LANG-ubn repository name prefix
	var repo_prefix string
	if strings.HasSuffix(ctx.Repo.Repository.Name, "-ubn") {
		repo_prefix = ctx.Repo.Repository.Name
	} else {
		char_index := strings.LastIndex(ctx.Repo.Repository.Name, "-ubn-")
		repo_prefix = ctx.Repo.Repository.Name[0 : char_index+4]
	}

	ctx.Data["username"] = ctx.Repo.Repository.Owner.Name
	ctx.Data["reponame"] = ctx.Repo.Repository.Name
	ctx.Data["RepoLink"] = ctx.Repo.Repository.Link()
	ctx.Data["Title"] = ctx.Tr("repo.hashtag.all_hashtags", ctx.Repo.Repository.Owner.Name+"/"+repo_prefix)
	ctx.Data["PageIsHashtags"] = true

	if results, err := models.GetHashtagSummary(repo_prefix, ctx.Repo.Repository.Owner.ID); err != nil {
		log.Error(4, "GetHashtagSummary(%s, %s): %v", repo_prefix, ctx.Repo.Repository.Owner.ID, err)
		ctx.Handle(http.StatusInternalServerError, "GetHashtagSummary", err)
		return
	} else {
		ctx.Data["Tags"] = results
	}

	ctx.HTML(200, HASHTAGS)
}

// HashtagDisambiguation produces a disambiguation page
func HashtagDisambiguation(ctx *context.Context) {
	hashtag := ctx.Params(":hashtag")

	// get the LANG-ubn repository name prefix
	var repo_prefix string
	if strings.HasSuffix(ctx.Repo.Repository.Name, "-ubn") {
		repo_prefix = ctx.Repo.Repository.Name
	} else {
		char_index := strings.LastIndex(ctx.Repo.Repository.Name, "-ubn-")
		repo_prefix = ctx.Repo.Repository.Name[0 : char_index+4]
	}

	ctx.Data["Repo"] = ctx.Repo.Repository
	ctx.Data["username"] = ctx.Repo.Repository.Owner.Name
	ctx.Data["reponame"] = ctx.Repo.Repository.Name
	ctx.Data["RepoLink"] = ctx.Repo.Repository.Link()
	ctx.Data["Title"] = ctx.Tr("repo.hashtag.hashtag_title", "#"+hashtag, repo_prefix)
	ctx.Data["Hashtag"] = hashtag
	ctx.Data["PageIsHashtags"] = true

	if results, err := models.GetHashtags(repo_prefix, ctx.Repo.Repository.Owner.ID, hashtag); err != nil {
		log.Error(4, "GetHashtagPages(%s, %s, %s): %v", repo_prefix, ctx.Repo.Repository.Owner.ID, hashtag, err)
		ctx.Handle(http.StatusInternalServerError, "GetHashtagPages", err)
		return
	} else {
		ctx.Data["Tags"] = results
	}

	ctx.HTML(200, HASHTAG)
}
