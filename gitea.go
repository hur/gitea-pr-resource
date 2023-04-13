package resource

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"code.gitea.io/sdk/gitea"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_gitea.go . Gitea
type Gitea interface {
	ListPullRequests(gitea.StateType) ([]*PullRequest, error)
	ListModifiedFiles(int64) ([]string, error)
	PostComment(string, string) error
	GetPullRequest(string, string) (*PullRequest, error)
	UpdateCommitStatus(string, string, string, string, string, string) error
}

// GiteaClient for handling API requests.
type GiteaClient struct {
	Client     *gitea.Client
	Repository string
	Owner      string
}

func NewGiteaClient(s *Source) (*GiteaClient, error) {
	owner, repository, err := parseRepository(s.Repository)
	if err != nil {
		return nil, err
	}

	client, err := gitea.NewClient(s.Endpoint, gitea.SetToken(s.AccessToken))
	if err != nil {
		return nil, err
	}

	return &GiteaClient{
		Client:     client,
		Repository: repository,
		Owner:      owner,
	}, nil
}

func (manager *GiteaClient) ListPullRequests(prStateFilter gitea.StateType) ([]*PullRequest, error) {
	var response []*PullRequest
	count := 0
	totalCount := -1
	page := 1
	for {
		prs, httpresponse, err := manager.Client.ListRepoPullRequests(
			manager.Owner,
			manager.Repository,
			gitea.ListPullRequestsOptions{
				ListOptions: gitea.ListOptions{
					Page:     page,
					PageSize: 100,
				},
				State: prStateFilter,
				Sort:  "recentupdate",
			},
		)

		if err != nil {
			return nil, fmt.Errorf("failed to list pull requests: %s", err)
		}

		for _, pr := range prs {
			commit, err := manager.getLatestCommitForPR(pr.Index)
			if err != nil {
				return nil, fmt.Errorf("failed to get latest commit for PR: %s", err)
			}
			response = append(response, &PullRequest{
				PullRequest: *pr,
				Tip:         *commit,
			})
		}

		count += len(prs)

		if page == 1 {
			xTotalCount := httpresponse.Header.Get("x-total-count")
			if xTotalCount == "" {
				return nil, errors.New("missing x-total-count header in Gitea API response")
			}

			totalCount, err = strconv.Atoi(xTotalCount)
			if err != nil {
				return nil, errors.New("failed to parse x-total-count header in Gitea API response")
			}

		}

		if count >= totalCount {
			break
		}

		page += 1
	}
	return response, nil
}

func (manager *GiteaClient) ListModifiedFiles(prNum int64) ([]string, error) {
	var files []string

	count := 0
	totalCount := -1
	page := 1
	for {
		changedFiles, httpresponse, err := manager.Client.ListPullRequestFiles(
			manager.Owner,
			manager.Repository,
			prNum,
			gitea.ListPullRequestFilesOptions{
				ListOptions: gitea.ListOptions{
					Page:     page,
					PageSize: 100,
				},
			},
		)

		if err != nil {
			return nil, fmt.Errorf("failed to list changed files in pull request: %s", err)
		}

		for _, file := range changedFiles {
			files = append(files, file.Filename)
		}

		count += len(changedFiles)

		if page == 1 {
			xTotalCount := httpresponse.Header.Get("x-total-count")
			if xTotalCount == "" {
				return nil, errors.New("missing x-total-count header in Gitea API response")
			}

			totalCount, err = strconv.Atoi(xTotalCount)
			if err != nil {
				return nil, errors.New("failed to parse x-total-count header in Gitea API response")
			}

		}

		if count >= totalCount {
			break
		}

		page += 1
	}

	return files, nil
}

func (manager *GiteaClient) GetPullRequest(prNumber, commitRef string) (*PullRequest, error) {
	prIndex, err := strconv.ParseInt(prNumber, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to convert pull request number to int: %s", err)
	}

	pr, _, err := manager.Client.GetPullRequest(manager.Owner, manager.Repository, prIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve pull request: %s", err)
	}
	count := 0
	page := 1
	totalCount := -1
	for {
		commits, httpResponse, err := manager.Client.ListPullRequestCommits(
			manager.Owner,
			manager.Repository,
			prIndex,
			gitea.ListPullRequestCommitsOptions{
				ListOptions: gitea.ListOptions{
					Page:     page,
					PageSize: 100,
				},
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve pull request commits: %s", err)
		}

		for _, commit := range commits {
			if commit.SHA == commitRef {
				return &PullRequest{
					PullRequest: *pr,
					Tip:         *commit,
				}, nil
			}
		}

		count += len(commits)

		if page == 1 {
			xTotalCount := httpResponse.Header.Get("x-total-count")
			if xTotalCount == "" {
				return nil, errors.New("missing x-total-count header in Gitea API response")
			}

			totalCount, err = strconv.Atoi(xTotalCount)
			if err != nil {
				return nil, errors.New("failed to parse x-total-count header in Gitea API response")
			}
		}

		if count >= totalCount {
			break
		}

		page += 1
	}
	// Return an error if the commit was not found
	return nil, fmt.Errorf("commit with ref '%s' does not exist", commitRef)
}

// PostComment to a pull request or issue.
func (manager *GiteaClient) PostComment(prNumber, comment string) error {
	prNum, err := strconv.ParseInt(prNumber, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to convert pull request number to int: %s", err)
	}

	_, _, err = manager.Client.CreateIssueComment(
		manager.Owner,
		manager.Repository,
		prNum,
		gitea.CreateIssueCommentOption{
			Body: comment,
		},
	)
	return err
}

// UpdateCommitStatus for a given commit (not supported by V4 API).
func (manager *GiteaClient) UpdateCommitStatus(commitRef, baseContext, statusContext, status, targetURL, description string) error {
	if baseContext == "" {
		baseContext = "concourse-ci"
	}

	if statusContext == "" {
		statusContext = "status"
	}

	if targetURL == "" {
		targetURL = strings.Join([]string{os.Getenv("ATC_EXTERNAL_URL"), "builds", os.Getenv("BUILD_ID")}, "/")
	}

	if description == "" {
		description = fmt.Sprintf("Concourse CI build %s", status)
	}

	_, _, err := manager.Client.CreateStatus(
		manager.Owner,
		manager.Repository,
		commitRef,
		gitea.CreateStatusOption{
			State:       gitea.StatusState(strings.ToLower(status)),
			TargetURL:   targetURL,
			Description: description,
			Context:     path.Join(baseContext, statusContext),
		})
	return err
}

func (manager GiteaClient) getLatestCommitForPR(prIndex int64) (*gitea.Commit, error) {
	commits, _, err := manager.Client.ListPullRequestCommits(
		manager.Owner,
		manager.Repository,
		prIndex,
		gitea.ListPullRequestCommitsOptions{
			ListOptions: gitea.ListOptions{},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve pull request commits: %s", err)
	}

	return commits[0], nil
}

func parseRepository(s string) (string, string, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return "", "", errors.New("malformed repository string")
	}
	return parts[0], parts[1], nil
}
