package resource_test

import (
	"testing"

	"code.gitea.io/sdk/gitea"
	resource "github.com/hur/gitea-pr-resource"
	"github.com/hur/gitea-pr-resource/fakes"
	"github.com/stretchr/testify/assert"
)

var (
	testPullRequests = []*resource.PullRequest{
		createTestPR(1, "master", true, false, nil, false, gitea.StateOpen),
		createTestPR(2, "master", false, false, nil, false, gitea.StateOpen),
		createTestPR(3, "master", false, false, nil, true, gitea.StateOpen),
		createTestPR(4, "master", false, false, nil, false, gitea.StateOpen),
		createTestPR(5, "master", false, true, nil, false, gitea.StateOpen),
		createTestPR(6, "master", false, false, nil, false, gitea.StateOpen),
		createTestPR(7, "develop", false, false, []string{"enhancement"}, false, gitea.StateOpen),
		createTestPR(8, "master", false, false, []string{"wontfix"}, false, gitea.StateOpen),
		createTestPR(9, "master", false, false, nil, false, gitea.StateOpen),
		createTestPR(10, "master", false, false, nil, false, gitea.StateClosed),
		createTestPR(12, "master", false, false, nil, false, gitea.StateOpen),
	}
)

func TestCheck(t *testing.T) {
	tests := []struct {
		description  string
		source       resource.Source
		version      resource.Version
		files        [][]string
		pullRequests []*resource.PullRequest
		expected     resource.CheckResponse
	}{
		{
			description: "check returns the latest version if there is no previous",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
			},
			version:      resource.Version{},
			pullRequests: testPullRequests,
			files:        [][]string{},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[1]),
			},
		},

		{
			description: "check returns the previous version when its still latest",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
			},
			version:      resource.NewVersion(testPullRequests[1]),
			pullRequests: testPullRequests,
			files:        [][]string{},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[1]),
			},
		},

		{
			description: "check returns all new versions since the last",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
			},
			version:      resource.NewVersion(testPullRequests[3]),
			pullRequests: testPullRequests,
			files:        [][]string{},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[2]),
				resource.NewVersion(testPullRequests[1]),
			},
		},

		{
			description: "check correctly ignores [skip ci] when specified",
			source: resource.Source{
				Repository:    "itsdalmo/test-repository",
				AccessToken:   "oauthtoken",
				DisableCISkip: true,
			},
			version:      resource.NewVersion(testPullRequests[1]),
			pullRequests: testPullRequests,
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[0]),
			},
		},

		{
			description: "check supports specifying base branch",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
				BaseBranch:  "develop",
			},
			version:      resource.Version{},
			pullRequests: testPullRequests,
			files:        [][]string{},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[6]),
			},
		},

		{
			description: "check returns latest version from a PR with at least one of the desired labels on it",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
				Labels:      []string{"enhancement"},
			},
			version:      resource.Version{},
			pullRequests: testPullRequests,
			files:        [][]string{},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[6]),
			},
		},

		{
			description: "check returns latest version from a PR with a single state filter",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
				State:       gitea.StateClosed,
			},
			version:      resource.Version{},
			pullRequests: testPullRequests,
			files:        [][]string{},
			expected: resource.CheckResponse{
				resource.NewVersion(testPullRequests[9]),
			},
		},

		{
			description: "check filters out versions from a PR which do not match the state filter",
			source: resource.Source{
				Repository:  "itsdalmo/test-repository",
				AccessToken: "oauthtoken",
				State:       gitea.StateOpen,
			},
			version:      resource.Version{},
			pullRequests: testPullRequests[9:10],
			files:        [][]string{},
			expected:     resource.CheckResponse(nil),
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			fakeGitea := new(fakes.FakeGitea)
			pullRequests := []*resource.PullRequest{}
			filterStates := gitea.StateOpen
			if tc.source.State != "" {
				filterStates = tc.source.State
			}
			if filterStates != gitea.StateAll {
				for i := range tc.pullRequests {
					if filterStates == tc.pullRequests[i].PullRequest.State {
						pullRequests = append(pullRequests, tc.pullRequests[i])
					}
				}
			} else {
				pullRequests = tc.pullRequests
			}
			fakeGitea.ListPullRequestsReturns(pullRequests, nil)

			input := resource.CheckRequest{Source: tc.source, Version: tc.version}
			output, err := resource.Check(input, fakeGitea)

			if assert.NoError(t, err) {
				assert.Equal(t, tc.expected, output)
			}
			assert.Equal(t, 1, fakeGitea.ListPullRequestsCallCount())
		})
	}
}

func TestContainsSkipCI(t *testing.T) {
	tests := []struct {
		description string
		message     string
		want        bool
	}{
		{
			description: "does not just match any symbol in the regexp",
			message:     "(",
			want:        false,
		},
		{
			description: "does not match when it should not",
			message:     "test",
			want:        false,
		},
		{
			description: "matches [ci skip]",
			message:     "[ci skip]",
			want:        true,
		},
		{
			description: "matches [skip ci]",
			message:     "[skip ci]",
			want:        true,
		},
		{
			description: "matches [no ci]",
			message:     "[no ci]",
			want:        true,
		},
		{
			description: "matches trailing skip ci",
			message:     "trailing [skip ci]",
			want:        true,
		},
		{
			description: "matches leading skip ci",
			message:     "[skip ci] leading",
			want:        true,
		},
		{
			description: "is case insensitive",
			message:     "case[Skip CI]insensitive",
			want:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			got := resource.ContainsSkipCI(tc.message)
			assert.Equal(t, tc.want, got)
		})
	}
}
