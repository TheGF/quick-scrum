package core

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type GoGit struct{}


var cloneUrl = regexp.MustCompile(`http.*\/([^\/]*?)(.git)?$`)

func (client GoGit) Clone(url string, path string) (string, error) {
	match := cloneUrl.FindStringSubmatch(url)
	if len(match) < 2 {
		return "Invalid url", ErrNoFound
	}
	name := match[1]
	path = filepath.Join(path, name)
	r, err := git.PlainClone(path, false, &git.CloneOptions{
		URL:               url,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})
	if err != nil {
		logrus.Warnf("Cannot clone git repository %s: %v", url, err)
		return "", nil
	}
	ref, err := r.Head()
	if err != nil {
		logrus.Warnf("Cannot retrieve git head for path %s: %v", path, err)
		return "", nil
	}
	return ref.Hash().String(), nil
}

func (client GoGit) GetStatus(project *Project) (GitStatus, error) {
	start := time.Now()
	gitFolder := filepath.Dir(project.Path)

	repo, err := git.PlainOpen(gitFolder)
	if err != nil {
		return GitStatus{}, err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return GitStatus{}, err
	}

	status, err := worktree.Status()
	if err != nil {
		return GitStatus{}, err
	}

	gitStatus := GitStatus{
		AshFiles: make([]string, 0),
		Files:    make(map[string]GitChange),
	}

	for name, state := range status {
		parts := strings.Split(name, string(os.PathSeparator))
		if len(parts) == 0 {
			continue
		}

		if parts[0] == ProjectFolder {
			if parts[1] == ProjectBoardsFolder || (project.Config.Public.IncludeLibInGit && parts[1] != ProjectLibraryFolder) {
				gitStatus.AshFiles = append(gitStatus.AshFiles, name)
			}
		} else {
			gitStatus.Files[name] = GitChange(state.Worktree)
		}
	}
	elapsed := time.Since(start)
	logrus.Infof("Git Status completed in %s", elapsed)
	return gitStatus, nil
}

func (client GoGit) Pull(project *Project, user string) (string, error) {
	start := time.Now()
	gitFolder := filepath.Dir(project.Path)

	r, err := git.PlainOpen(gitFolder)
	if err != nil {
		return "", err
	}
	logrus.Debugf("Open git repository %s", gitFolder)

	w, err := r.Worktree()
	if err != nil {
		return "", err
	}
	logrus.Debugf("Worktree successfully open")

	err = w.Pull(&git.PullOptions{RemoteName: "origin"})
	if err != nil {
		return "", err
	}

	// Print the latest commit that was just pulled
	ref, err := r.Head()
	if err != nil {
		return "", err
	}

	commit, err := r.CommitObject(ref.Hash())
	if err != nil {
		return "", err
	}

	elapsed := time.Since(start)
	logrus.Infof("Pull completed in %s", elapsed)
	return fmt.Sprintf("Pull completed in %s. Last commit id: %s", elapsed, commit.String()), nil
}

func (client GoGit) Push(project *Project, user string) (string, error) {
	start := time.Now()
	gitFolder := filepath.Dir(project.Path)

	r, err := git.PlainOpen(gitFolder)
	if err != nil {
		return "", err
	}

	auth, err := getAuth(project, user)
	if err != nil {
		return "", err
	}

	err = r.Push(&git.PushOptions{
		Auth: auth,
	})
	if err != nil {
		return "", err
	}
	elapsed := time.Since(start)
	logrus.Infof("Push completed in %s", elapsed)
	return fmt.Sprintf("Push completed in %s", elapsed), nil
}

func (client GoGit) Commit(project *Project, commitInfo CommitInfo) (string, error) {
	start := time.Now()
	gitFolder := filepath.Dir(project.Path)
	userInfo, err := GetUserInfo(project, commitInfo.User)
	if err != nil {
		return "", err
	}

	r, err := git.PlainOpen(gitFolder)
	if err != nil {
		return "", err
	}
	logrus.Debugf("Open git repository %s", gitFolder)

	w, err := r.Worktree()
	if err != nil {
		return "", err
	}
	logrus.Debugf("Worktree successfully open")

	message := prepareGitMessage(commitInfo)
	logrus.Debugf("Git message:\n%s", message)

	for _, file := range commitInfo.Files {
		if _, err = w.Add(file); err != nil {
			logrus.Warnf("Cannot add file %s to the commit: %v", file, err)
		} else {
			logrus.Debugf("Added file %s to commit", file)
		}
	}

	hash, err := w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  commitInfo.User,
			Email: userInfo.Email,
			When:  time.Now(),
		},
	})
	if err != nil {
		logrus.Warnf("Cannot complete commit: %v", err)
		return "", err
	}
	elapsed := time.Since(start)
	logrus.Infof("Commit completed in %s. Hash: %v", elapsed, hash)
	return fmt.Sprintf("Commit completed in %s. Hash: %s", elapsed, hash.String()), nil
}
