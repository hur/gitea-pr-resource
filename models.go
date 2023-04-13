package resource

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"code.gitea.io/sdk/gitea"
)

// Source represents the configuration for the resource.
type Source struct {
	Repository    string          `json:"repository"`
	Endpoint      string          `json:"endpoint"`
	AccessToken   string          `json:"access_token"`
	Paths         []string        `json:"paths"`
	IgnorePaths   []string        `json:"ignore_paths"`
	State         gitea.StateType `json:"state"`
	DisableCISkip bool            `json:"disable_ci_skip"`
	BaseBranch    string          `json:"base_branch"`
	Labels        []string        `json:"labels"`
}

func (s *Source) Validate() error {
	if s.AccessToken == "" {
		return errors.New("access_token must be set")
	}
	if s.Repository == "" {
		return errors.New("repository must be set")
	}
	if s.Endpoint != "" {
		return errors.New("endpoint must be set")
	}

	switch s.State {
	case gitea.StateOpen:
	case gitea.StateClosed:
	case gitea.StateAll:
	default:
		return errors.New(fmt.Sprintf("state value \"%s\" must be one of: open, closed, all", s.State))
	}

	return nil
}

// Resource version for concourse
type Version struct {
	PR            string          `json:"pr"`
	Commit        string          `json:"commit"`
	CommittedDate time.Time       `json:"committed,omitempty"`
	State         gitea.StateType `json:"state"`
}

func NewVersion(pr *PullRequest) Version {
	return Version{
		PR:            strconv.FormatInt(pr.Index, 10),
		Commit:        pr.Head.Sha,
		CommittedDate: pr.UpdatedDate().UTC(), // Unlike Github, Gitea doesn't normalize timestamps to UTC
		State:         pr.State,
	}
}

// PullRequest represents a pull request and includes the tip (commit).
type PullRequest struct {
	gitea.PullRequest
	Tip gitea.Commit
}

// UpdatedDate returns the last time a PR was updated, either by commit
// or being closed/merged.
func (pr *PullRequest) UpdatedDate() time.Time {
	//date := p.Tip.CommittedDate
	if pr.State == gitea.StateClosed && pr.HasMerged {
		return *pr.Merged
	}

	if pr.State == gitea.StateClosed && !pr.HasMerged {
		return *pr.Closed
	}

	//return *pr.Updated
	return pr.Tip.Created
}

// Metadata output from get/put steps.
type Metadata []*MetadataField

// Add a MetadataField to the Metadata.
func (m *Metadata) Add(name, value string) {
	*m = append(*m, &MetadataField{Name: name, Value: value})
}

// MetadataField ...
type MetadataField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
