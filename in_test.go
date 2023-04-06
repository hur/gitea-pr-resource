package resource_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"code.gitea.io/sdk/gitea"
	resource "github.com/hur/gitea-pr-resource"
	"github.com/hur/gitea-pr-resource/fakes"
	"github.com/stretchr/testify/assert"
)

func TestGet(t *testing.T) {

	tests := []struct {
		description    string
		source         resource.Source
		version        resource.Version
		parameters     resource.GetParameters
		pullRequest    *resource.PullRequest
		versionString  string
		metadataString string
		filesString    string
	}{
		{
			description: "get works",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
			},
			version: resource.Version{
				PR:            "pr1",
				Commit:        "commit1",
				CommittedDate: time.Time{},
				State:         gitea.StateOpen,
			},
			parameters:     resource.GetParameters{},
			pullRequest:    createTestPR(1, "master", false, false, nil, false, gitea.StateOpen),
			versionString:  `{"pr":"pr1","commit":"commit1","committed":"0001-01-01T00:00:00Z","state":"open"}`,
			metadataString: `[{"name":"pr","value":"1"},{"name":"title","value":"pr1 title"},{"name":"url","value":"pr1 url"},{"name":"head_name","value":"pr1"},{"name":"head_sha","value":"oid1"},{"name":"base_name","value":"master"},{"name":"base_sha","value":"sha"},{"name":"message","value":"commit message1"},{"name":"author","value":"login1"},{"name":"author_email","value":"user@example.com"},{"name":"state","value":"open"}]`,
		},
		{
			description: "get supports rebasing",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
			},
			version: resource.Version{
				PR:            "pr1",
				Commit:        "commit1",
				CommittedDate: time.Time{},
				State:         gitea.StateOpen,
			},
			parameters: resource.GetParameters{
				IntegrationTool: "rebase",
			},
			pullRequest:    createTestPR(1, "master", false, false, nil, false, gitea.StateOpen),
			versionString:  `{"pr":"pr1","commit":"commit1","committed":"0001-01-01T00:00:00Z","state":"open"}`,
			metadataString: `[{"name":"pr","value":"1"},{"name":"title","value":"pr1 title"},{"name":"url","value":"pr1 url"},{"name":"head_name","value":"pr1"},{"name":"head_sha","value":"oid1"},{"name":"base_name","value":"master"},{"name":"base_sha","value":"sha"},{"name":"message","value":"commit message1"},{"name":"author","value":"login1"},{"name":"author_email","value":"user@example.com"},{"name":"state","value":"open"}]`,
		},
		{
			description: "get supports checkout",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
			},
			version: resource.Version{
				PR:            "pr1",
				Commit:        "commit1",
				CommittedDate: time.Time{},
				State:         gitea.StateOpen,
			},
			parameters: resource.GetParameters{
				IntegrationTool: "checkout",
			},
			pullRequest:    createTestPR(1, "master", false, false, nil, false, gitea.StateOpen),
			versionString:  `{"pr":"pr1","commit":"commit1","committed":"0001-01-01T00:00:00Z","state":"open"}`,
			metadataString: `[{"name":"pr","value":"1"},{"name":"title","value":"pr1 title"},{"name":"url","value":"pr1 url"},{"name":"head_name","value":"pr1"},{"name":"head_sha","value":"oid1"},{"name":"base_name","value":"master"},{"name":"base_sha","value":"sha"},{"name":"message","value":"commit message1"},{"name":"author","value":"login1"},{"name":"author_email","value":"user@example.com"},{"name":"state","value":"open"}]`,
		},
		{
			description: "get supports git_depth",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
			},
			version: resource.Version{
				PR:            "pr1",
				Commit:        "commit1",
				CommittedDate: time.Time{},
				State:         gitea.StateOpen,
			},
			parameters: resource.GetParameters{
				GitDepth: 2,
			},
			pullRequest:    createTestPR(1, "master", false, false, nil, false, gitea.StateOpen),
			versionString:  `{"pr":"pr1","commit":"commit1","committed":"0001-01-01T00:00:00Z","state":"open"}`,
			metadataString: `[{"name":"pr","value":"1"},{"name":"title","value":"pr1 title"},{"name":"url","value":"pr1 url"},{"name":"head_name","value":"pr1"},{"name":"head_sha","value":"oid1"},{"name":"base_name","value":"master"},{"name":"base_sha","value":"sha"},{"name":"message","value":"commit message1"},{"name":"author","value":"login1"},{"name":"author_email","value":"user@example.com"},{"name":"state","value":"open"}]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			gitea := new(fakes.FakeGitea)
			gitea.GetPullRequestReturns(tc.pullRequest, nil)

			git := new(fakes.FakeGit)
			git.RevParseReturns("sha", nil)

			dir := createTestDirectory(t)
			defer os.RemoveAll(dir)

			input := resource.GetRequest{Source: tc.source, Version: tc.version, Params: tc.parameters}
			output, err := resource.Get(input, gitea, git, dir)

			// Validate output
			if assert.NoError(t, err) {
				assert.Equal(t, tc.version, output.Version)

				// Verify written files
				version := readTestFile(t, filepath.Join(dir, ".git", "resource", "version.json"))
				assert.Equal(t, tc.versionString, version)

				metadata := readTestFile(t, filepath.Join(dir, ".git", "resource", "metadata.json"))
				assert.Equal(t, tc.metadataString, metadata)

				// Verify individual files
				files := map[string]string{
					"pr":           "1",
					"url":          "pr1 url",
					"head_name":    "pr1",
					"head_sha":     "oid1",
					"base_name":    "master",
					"base_sha":     "sha",
					"message":      "commit message1",
					"author":       "login1",
					"author_email": "user@example.com",
					"title":        "pr1 title",
				}

				for filename, expected := range files {
					actual := readTestFile(t, filepath.Join(dir, ".git", "resource", filename))
					assert.Equal(t, expected, actual)
				}

			}

			// Validate Github calls
			if assert.Equal(t, 1, gitea.GetPullRequestCallCount()) {
				pr, commit := gitea.GetPullRequestArgsForCall(0)
				assert.Equal(t, tc.version.PR, pr)
				assert.Equal(t, tc.version.Commit, commit)
			}

			// Validate Git calls
			if assert.Equal(t, 1, git.InitCallCount()) {
				base := git.InitArgsForCall(0)
				assert.Equal(t, tc.pullRequest.Base.Ref, base)
			}

			if assert.Equal(t, 1, git.PullCallCount()) {
				url, base, depth, submodules, fetchTags := git.PullArgsForCall(0)
				assert.Equal(t, tc.pullRequest.Head.Repository.CloneURL, url)
				assert.Equal(t, tc.pullRequest.Base.Ref, base)
				assert.Equal(t, tc.parameters.GitDepth, depth)
				assert.Equal(t, tc.parameters.Submodules, submodules)
				assert.Equal(t, tc.parameters.FetchTags, fetchTags)
			}

			if assert.Equal(t, 1, git.RevParseCallCount()) {
				base := git.RevParseArgsForCall(0)
				assert.Equal(t, tc.pullRequest.Base.Ref, base)
			}

			if assert.Equal(t, 1, git.FetchCallCount()) {
				url, pr, depth, submodules := git.FetchArgsForCall(0)
				assert.Equal(t, tc.pullRequest.Head.Repository.CloneURL, url)
				assert.Equal(t, tc.pullRequest.Index, int64(pr))
				assert.Equal(t, tc.parameters.GitDepth, depth)
				assert.Equal(t, tc.parameters.Submodules, submodules)
			}

			switch tc.parameters.IntegrationTool {
			case "rebase":
				if assert.Equal(t, 1, git.RebaseCallCount()) {
					branch, tip, submodules := git.RebaseArgsForCall(0)
					assert.Equal(t, tc.pullRequest.Base.Ref, branch)
					assert.Equal(t, tc.pullRequest.Tip.SHA, tip)
					assert.Equal(t, tc.parameters.Submodules, submodules)
				}
			case "checkout":
				if assert.Equal(t, 1, git.CheckoutCallCount()) {
					branch, sha, submodules := git.CheckoutArgsForCall(0)
					assert.Equal(t, tc.pullRequest.Head.Ref, branch)
					assert.Equal(t, tc.pullRequest.Tip.SHA, sha)
					assert.Equal(t, tc.parameters.Submodules, submodules)
				}
			default:
				if assert.Equal(t, 1, git.MergeCallCount()) {
					tip, submodules := git.MergeArgsForCall(0)
					assert.Equal(t, tc.pullRequest.Tip.SHA, tip)
					assert.Equal(t, tc.parameters.Submodules, submodules)
				}
			}
		})
	}
}

func TestGetSkipDownload(t *testing.T) {

	tests := []struct {
		description string
		source      resource.Source
		version     resource.Version
		parameters  resource.GetParameters
	}{
		{
			description: "skip download works",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
			},
			version: resource.Version{
				PR:            "pr1",
				Commit:        "commit1",
				CommittedDate: time.Time{},
			},
			parameters: resource.GetParameters{SkipDownload: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			github := new(fakes.FakeGitea)
			git := new(fakes.FakeGit)
			dir := createTestDirectory(t)
			defer os.RemoveAll(dir)

			// Run the get and check output
			input := resource.GetRequest{Source: tc.source, Version: tc.version, Params: tc.parameters}
			output, err := resource.Get(input, github, git, dir)

			if assert.NoError(t, err) {
				assert.Equal(t, tc.version, output.Version)
			}
		})
	}
}

func createTestPR(
	count int,
	baseName string,
	skipCI bool,
	isCrossRepo bool,
	labels []string,
	isDraft bool,
	state gitea.StateType,
) *resource.PullRequest {
	n := strconv.Itoa(count)
	d := time.Now().AddDate(0, 0, -count)
	m := fmt.Sprintf("commit message%s", n)
	if skipCI {
		m = "[skip ci]" + m
	}

	var labelObjects []*gitea.Label
	for _, l := range labels {
		lObject := gitea.Label{
			Name: l,
		}

		labelObjects = append(labelObjects, &lObject)
	}

	hasMerged := false
	if state == gitea.StateClosed {
		hasMerged = true
	}
	/*
		ID:          fmt.Sprintf("pr%s", n),
		Number:      count,
		Title:       fmt.Sprintf("pr%s title", n),
		URL:         fmt.Sprintf("pr%s url", n),
		BaseRefName: baseName,
		HeadRefName: fmt.Sprintf("pr%s", n),
		Repository: struct{ URL string }{
			URL: fmt.Sprintf("repo%s url", n),
		},
		IsCrossRepository: isCrossRepo,
		IsDraft:           isDraft,
		State:             state,
		ClosedAt:          githubv4.DateTime{Time: time.Now()},
		MergedAt:          githubv4.DateTime{Time: time.Now()},
	*/
	return &resource.PullRequest{
		PullRequest: gitea.PullRequest{
			URL:   fmt.Sprintf("pr%s url", n),
			Index: int64(count),
			Title: fmt.Sprintf("pr%s title", n),
			Base: &gitea.PRBranchInfo{
				Name: baseName,
				Ref:  baseName,
				Repository: &gitea.Repository{
					CloneURL: fmt.Sprintf("pr%s url", n),
				},
			},
			Head: &gitea.PRBranchInfo{
				Name: fmt.Sprintf("pr%s", n),
				Ref:  fmt.Sprintf("pr%s", n),
				Repository: &gitea.Repository{
					CloneURL: fmt.Sprintf("pr%s url", n),
				},
			},
			Labels:    labelObjects,
			State:     state,
			Closed:    ptr(time.Now()),
			Merged:    ptr(time.Now()),
			HasMerged: hasMerged,
		},
		/*
			ID:            fmt.Sprintf("commit%s", n),
			OID:           fmt.Sprintf("oid%s", n),
			CommittedDate: d,
			Message:       m,
			Author: struct {
				User  struct{ Login string }
				Email string
			}{
				User: struct{ Login string }{
					Login: fmt.Sprintf("login%s", n),
				},
				Email: "user@example.com",
			},
		*/
		Tip: gitea.Commit{
			CommitMeta: &gitea.CommitMeta{
				Created: d,
				SHA:     fmt.Sprintf("oid%s", n),
			},
			RepoCommit: &gitea.RepoCommit{
				Message: m,
				Author: &gitea.CommitUser{
					Identity: gitea.Identity{
						Name:  fmt.Sprintf("login%s", n),
						Email: "user@example.com",
					},
				},
			},
			//Author: &gitea.User{
			//	ID:       int64(count),
			//	Email:    "user@example.com",
			//	UserName: fmt.Sprintf("login%s", n),
			//},
		},
	}
}

func createTestDirectory(t *testing.T) string {
	dir, err := ioutil.TempDir("", "github-pr-resource")
	if err != nil {
		t.Fatalf("failed to create temporary directory")
	}
	return dir
}

func readTestFile(t *testing.T, path string) string {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read: %s: %s", path, err)
	}
	return string(b)
}

func ptr(t time.Time) *time.Time {
	return &t
}
