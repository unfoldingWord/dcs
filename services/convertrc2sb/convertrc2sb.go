// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convertrc2sb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	rc2sb "github.com/unfoldingWord/go-rc2sb"
	ts2rc "github.com/unfoldingWord/go-ts2rc"
)

// RepoQualifiesForConversion checks if a repo meets all criteria for SB conversion:
// 1. CONVERT_RC2SB_TOPICS is configured in app.ini (non-empty)
// 2. DefaultBranch is "master"
// 3. Repo has at least one of the configured topics
// 4. DefaultBranch DM MetadataType is "rc" or "ts" (ts repos are first converted to RC)
func RepoQualifiesForConversion(ctx context.Context, repo *repo_model.Repository) (bool, error) {
	if repo == nil {
		return false, nil
	}

	if len(setting.DCS.ConvertRC2SBTopics) == 0 {
		log.Debug("ConvertRC2SB: CONVERT_RC2SB_TOPICS not configured, skipping all conversions")
		return false, nil
	}

	if repo.DefaultBranch != "master" {
		log.Debug("ConvertRC2SB: %s does not qualify — DefaultBranch is %q (need \"master\")", repo.FullName(), repo.DefaultBranch)
		return false, nil
	}

	if !repoHasQualifyingTopic(repo) {
		log.Debug("ConvertRC2SB: %s does not qualify — no qualifying topic (has: %v)", repo.FullName(), repo.Topics)
		return false, nil
	}

	// Check for a default-branch DM with metadata_type = "rc" or "ts", regardless of validation errors
	hasConvertibleMetadata, err := repo_model.HasDefaultBranchConvertibleMetadata(ctx, repo.ID)
	if err != nil {
		return false, fmt.Errorf("HasDefaultBranchConvertibleMetadata: %w", err)
	}
	if !hasConvertibleMetadata {
		log.Debug("ConvertRC2SB: %s does not qualify — no default-branch DM with metadata_type=rc or ts", repo.FullName())
		return false, nil
	}

	log.Info("ConvertRC2SB: %s qualifies for conversion", repo.FullName())
	return true, nil
}

// repoHasQualifyingTopic checks if repo.Topics contains at least one qualifying topic.
func repoHasQualifyingTopic(repo *repo_model.Repository) bool {
	for _, topic := range repo.Topics {
		for _, qt := range setting.DCS.ConvertRC2SBTopics {
			if strings.EqualFold(topic, qt) {
				return true
			}
		}
	}
	return false
}

// ForBranch converts an RC or ts repo at the HEAD of the given branch to SB format
// and pushes the result to the "main" branch. ts repos are first converted to RC
// format via ts2rc, then follow the same RC-to-SB pipeline.
func ForBranch(ctx context.Context, repo *repo_model.Repository, branchName string) error {
	if repo == nil {
		return errors.New("repo must not be nil")
	}

	log.Info("ConvertRC2SB: starting conversion for %s branch %s", repo.FullName(), branchName)

	// Load owner for commit identity
	if err := repo.LoadOwner(ctx); err != nil {
		return fmt.Errorf("LoadOwner: %w", err)
	}

	// Create temp directory for conversion work
	tmpDir, cleanup, err := setting.AppDataTempDir("repo-rc2sb-convert").MkdirTempRandom(fmt.Sprintf("%d-", repo.ID))
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer cleanup()

	dm := getConversionDM(ctx, repo)
	isTS := dm != nil && dm.MetadataType == "ts"

	// Step 1: Shallow clone at the branch HEAD.
	// ts repos are cloned to a separate dir and first converted to RC format below.
	rcDir := filepath.Join(tmpDir, "rc")
	cloneDir := rcDir
	if isTS {
		cloneDir = filepath.Join(tmpDir, "ts")
	}
	if err := cloneAtRef(ctx, repo.RepoPath(), branchName, cloneDir); err != nil {
		return fmt.Errorf("clone at branch %s: %w", branchName, err)
	}

	// Step 1b: For ts repos, convert ts -> RC so the RC -> SB pipeline below applies unchanged.
	if isTS {
		twSourceDir, err := prepareTsTWSourceDir(ctx, tmpDir, repo, dm)
		if err != nil {
			return fmt.Errorf("prepareTsTWSourceDir: %w", err)
		}
		rep := ts2rc.Convert(ctx, cloneDir, rcDir, ts2rc.Options{TWSourceDir: twSourceDir})
		if !rep.OK {
			return fmt.Errorf("ts2rc.Convert: %s", rep.Error)
		}
		for _, warning := range rep.Warnings {
			log.Warn("ConvertRC2SB: ts2rc warning for %s: %s", repo.FullName(), warning)
		}
		log.Info("ConvertRC2SB: ts2rc conversion successful for %s — class=%s, package_version=%d",
			repo.FullName(), rep.Class, rep.Version)
	}

	// Step 2: Prepare payload for TWL and TW repos.
	// For TWL repos: returns a path to the TW clone; passed to rc2sb as PayloadPath.
	// For TW repos:  copies the TWL clone directly into rcDir/<lang>_twl/ so the
	//                library can auto-detect it from inDir; returns "".
	payloadPath, err := preparePayloadPath(ctx, tmpDir, rcDir, repo, dm)
	if err != nil {
		return fmt.Errorf("preparePayloadPath: %w", err)
	}

	// Step 3: Run rc2sb conversion
	sbDir := filepath.Join(tmpDir, "sb")
	result, err := rc2sb.Convert(ctx, rcDir, sbDir, rc2sb.Options{
		PayloadPath: payloadPath,
	})
	if err != nil {
		return fmt.Errorf("rc2sb.Convert: %w", err)
	}
	log.Info("ConvertRC2SB: conversion successful for %s — subject=%s, ingredients=%d",
		repo.FullName(), result.Subject, result.Ingredients)

	// Step 4: Clone the full repo to a working directory for pushing
	workDir := filepath.Join(tmpDir, "work")
	if err := git.Clone(ctx, repo.RepoPath(), workDir, git.CloneRepoOptions{Quiet: true}); err != nil {
		return fmt.Errorf("clone work dir: %w", err)
	}

	// Step 5: Create or checkout the "main" branch
	if err := prepareMainBranch(ctx, workDir, branchName); err != nil {
		return fmt.Errorf("prepareMainBranch: %w", err)
	}

	// Step 6: Remove all files in working tree (except .git)
	if err := clearWorkingTree(workDir); err != nil {
		return fmt.Errorf("clearWorkingTree: %w", err)
	}

	// Step 7: Copy converted SB files into working tree
	if err := copyDir(sbDir, workDir); err != nil {
		return fmt.Errorf("copy SB files: %w", err)
	}

	// Step 8: Stage all changes
	if _, _, err := gitcmd.NewCommand("add", "-A").WithDir(workDir).RunStdString(ctx); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Check if there are any changes to commit
	stdout, _, err := gitcmd.NewCommand("status", "--porcelain").WithDir(workDir).RunStdString(ctx)
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if strings.TrimSpace(stdout) == "" {
		log.Info("ConvertRC2SB: no changes to commit for %s branch %s", repo.FullName(), branchName)
		return nil
	}

	// Step 9: Commit
	commitMsg := "Convert RC to SB from branch " + branchName
	if isTS {
		commitMsg = "Convert tS to SB from branch " + branchName
	}
	doer := repo.Owner
	sig := doer.NewGitSig()

	_, _, err = gitcmd.NewCommand("commit",
		"-m").AddDynamicArguments(commitMsg).
		AddArguments("--author").AddDynamicArguments(fmt.Sprintf("%s <%s>", sig.Name, sig.Email)).
		WithDir(workDir).RunStdString(ctx)
	if err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	// Step 10: Push to the bare repo
	// Use PushingEnvironment (not InternalPushingEnvironment) so hooks fire
	// and Door43Metadata is processed for the new main branch
	env := repo_module.PushingEnvironment(doer, repo)
	if err := git.Push(ctx, workDir, git.PushOptions{
		Remote: repo.RepoPath(),
		Branch: "main",
		Env:    env,
	}); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	log.Info("ConvertRC2SB: successfully pushed SB content to main branch for %s", repo.FullName())
	return nil
}

// ConvertRC2SBAllRepos finds all qualifying repos and converts their default (master) branch.
func ConvertRC2SBAllRepos(ctx context.Context) error { //nolint:revive // name is used by cron task reference
	log.Trace("Doing: ConvertRC2SBAllRepos")

	if len(setting.DCS.ConvertRC2SBTopics) == 0 {
		log.Debug("ConvertRC2SBAllRepos: CONVERT_RC2SB_TOPICS not configured, skipping")
		return nil
	}

	repos, err := repo_model.GetReposQualifiedForRC2SBConversion(ctx, setting.DCS.ConvertRC2SBTopics)
	if err != nil {
		return fmt.Errorf("GetReposQualifiedForRC2SBConversion: %w", err)
	}

	log.Info("ConvertRC2SBAllRepos: found %d qualifying repos", len(repos))

	for _, repo := range repos {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := ForBranch(ctx, repo, repo.DefaultBranch); err != nil {
			log.Error("ConvertRC2SBAllRepos: conversion failed for %s branch %s: %v", repo.FullName(), repo.DefaultBranch, err)
			if noticeErr := system_model.CreateRepositoryNotice(
				"ConvertRC2SB failed for repository (%s) branch (%s): %v", repo.FullName(), repo.DefaultBranch, err,
			); noticeErr != nil {
				log.Error("CreateRepositoryNotice: %v", noticeErr)
			}
			continue
		}
	}

	log.Trace("Finished: ConvertRC2SBAllRepos")
	return nil
}

// cloneAtRef does a shallow clone of the repo at the specified branch or tag ref.
func cloneAtRef(ctx context.Context, repoPath, ref, destination string) error {
	err := git.Clone(ctx, repoPath, destination, git.CloneRepoOptions{
		Quiet:  true,
		Depth:  1,
		Branch: ref,
	})
	if err != nil {
		// Fallback to full clone if shallow clone fails
		if removeErr := util.RemoveAll(destination); removeErr != nil {
			log.Warn("cloneAtRef: failed to clean shallow clone destination: %v", removeErr)
		}
		if err := git.Clone(ctx, repoPath, destination, git.CloneRepoOptions{Quiet: true}); err != nil {
			return err
		}
		// Checkout the ref
		_, _, checkoutErr := gitcmd.NewCommand("checkout").AddDynamicArguments(ref).
			WithDir(destination).RunStdString(ctx)
		if checkoutErr != nil {
			return fmt.Errorf("checkout ref %s: %w", ref, checkoutErr)
		}
	}
	return nil
}

// prepareMainBranch creates or checks out the "main" branch in the working directory.
//
// The repo is cloned fresh for each conversion, so an existing "main" branch is present
// only as the remote-tracking ref refs/remotes/origin/main, never as a local
// refs/heads/main. When it exists we branch from origin/main so the new SB commit
// fast-forwards onto main's accumulated conversion history; otherwise we create main
// from the source (RC) branch for the first-ever conversion.
func prepareMainBranch(ctx context.Context, workDir, branchName string) error {
	// Does the remote already have a main branch?
	_, _, err := gitcmd.NewCommand("rev-parse", "--verify", "refs/remotes/origin/main").
		WithDir(workDir).RunStdString(ctx)
	if err != nil {
		// No existing main — create it from the source branch (first conversion).
		_, _, err = gitcmd.NewCommand("checkout", "-b", "main").AddDynamicArguments(branchName).
			WithDir(workDir).RunStdString(ctx)
		if err != nil {
			return fmt.Errorf("create main branch from %s: %w", branchName, err)
		}
		return nil
	}

	// main exists on the remote — branch from origin/main so commits fast-forward onto it.
	_, _, err = gitcmd.NewCommand("checkout", "-b", "main", "origin/main").
		WithDir(workDir).RunStdString(ctx)
	if err != nil {
		return fmt.Errorf("checkout main from origin/main: %w", err)
	}
	return nil
}

// clearWorkingTree removes all files and directories in the working tree except .git.
func clearWorkingTree(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// copyDir recursively copies src directory contents into dst directory.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// getConversionDM returns the Door43Metadata to base conversion decisions on:
// the default-branch DM if present, otherwise the repo-level DM, otherwise nil.
func getConversionDM(ctx context.Context, repo *repo_model.Repository) *repo_model.Door43Metadata {
	if err := repo.LoadLatestDMs(ctx); err != nil {
		log.Warn("ConvertRC2SB: LoadLatestDMs failed for %s: %v", repo.FullName(), err)
		return nil
	}
	dm := repo.DefaultBranchDM
	if dm == nil {
		dm = repo.RepoDM
	}
	return dm
}

// preparePayloadPath prepares the payload for TWL, OBS TWL, and TW repos.
//
// For TWL repos (Subject "TSV Translation Words Links") and OBS TWL repos
// (Subject "TSV OBS Translation Words Links"): clones the corresponding TW repo
// (<lang>_tw) into tmpDir/payload and returns that path. The rc2sb library receives it
// via PayloadPath and copies bible/ into ingredients/payload/, rewriting the rc:// links
// in the twl_*.tsv / twl_OBS.tsv file to ./payload/ paths.
//
// For TW repos (Subject "Translation Words"): clones the corresponding TWL repo and copies
// it directly into rcDir/<lang>_twl/ so the rc2sb library can auto-detect it from inDir
// (the same pattern the TWL handler uses to auto-detect <lang>_tw/). Returns "".
//
// Returns ("", nil) when no payload is needed.
func preparePayloadPath(ctx context.Context, tmpDir, rcDir string, repo *repo_model.Repository, dm *repo_model.Door43Metadata) (string, error) {
	if dm == nil || dm.Language == "" {
		return "", nil
	}

	var payloadRepoName string
	isTWRepo := false
	switch dm.Subject {
	case "TSV Translation Words Links", "TSV OBS Translation Words Links":
		payloadRepoName = dm.Language + "_tw"
	case "Translation Words":
		payloadRepoName = dm.Language + "_twl"
		isTWRepo = true
	default:
		return "", nil
	}

	payloadRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, repo.OwnerName, payloadRepoName)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("GetRepositoryByOwnerAndName(%s): %w", payloadRepoName, err)
	}

	payloadGitRepo, err := gitrepo.OpenRepository(ctx, payloadRepo)
	if err != nil {
		return "", fmt.Errorf("OpenRepository(%s): %w", payloadRepo.FullName(), err)
	}
	defer payloadGitRepo.Close()

	commitID, err := payloadGitRepo.ConvertToGitID(payloadRepo.DefaultBranch)
	if err != nil {
		return "", fmt.Errorf("resolve %s default branch: %w", payloadRepo.FullName(), err)
	}

	payloadDir := filepath.Join(tmpDir, "payload")
	if err := cloneAtRef(ctx, payloadRepo.RepoPath(), payloadRepo.DefaultBranch, payloadDir); err != nil {
		_ = util.RemoveAll(payloadDir)
		if err := git.Clone(ctx, payloadRepo.RepoPath(), payloadDir, git.CloneRepoOptions{Quiet: true}); err != nil {
			return "", fmt.Errorf("clone payload repo %s: %w", payloadRepo.FullName(), err)
		}
		_, _, checkoutErr := gitcmd.NewCommand("checkout", "--detach").AddDynamicArguments(commitID.String()).
			WithDir(payloadDir).RunStdString(ctx)
		if checkoutErr != nil {
			return "", fmt.Errorf("checkout %s commit: %w", payloadRepo.FullName(), checkoutErr)
		}
	}

	if isTWRepo {
		// Copy the TWL clone into rcDir/<lang>_twl/ so the library auto-detects it in inDir.
		destDir := filepath.Join(rcDir, payloadRepoName)
		if err := copyDir(payloadDir, destDir); err != nil {
			return "", fmt.Errorf("copy TWL payload into rcDir: %w", err)
		}
		return "", nil
	}

	return payloadDir, nil
}

// tsTWSourceOwners are tried in order when locating a canonical en_tw repo. ts2rc uses it
// to place Translation Words articles under bible/{kt|names|other}/ by matching slug filenames.
var tsTWSourceOwners = []string{"unfoldingWord", "Door43-Catalog"}

// prepareTsTWSourceDir clones a canonical en_tw repo into tmpDir/tw-source for ts
// Translation Words conversions and returns its path. Returns ("", nil) when the repo
// is not a ts TW repo or no canonical en_tw repo exists on this server — ts2rc then
// falls back to writing articles under bible/other/ with a warning.
func prepareTsTWSourceDir(ctx context.Context, tmpDir string, repo *repo_model.Repository, dm *repo_model.Door43Metadata) (string, error) {
	if dm == nil || dm.Subject != "Translation Words" {
		return "", nil
	}

	for _, owner := range tsTWSourceOwners {
		twRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, owner, "en_tw")
		if err != nil {
			if repo_model.IsErrRepoNotExist(err) {
				continue
			}
			return "", fmt.Errorf("GetRepositoryByOwnerAndName(%s/en_tw): %w", owner, err)
		}
		twSourceDir := filepath.Join(tmpDir, "tw-source")
		if err := cloneAtRef(ctx, twRepo.RepoPath(), twRepo.DefaultBranch, twSourceDir); err != nil {
			return "", fmt.Errorf("clone TW source repo %s: %w", twRepo.FullName(), err)
		}
		return twSourceDir, nil
	}

	log.Warn("ConvertRC2SB: no canonical en_tw repo found for TW category lookup; %s articles will fall back to bible/other/", repo.FullName())
	return "", nil
}
