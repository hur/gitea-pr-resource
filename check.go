package resource

import (
	"regexp"
	"sort"
)

type CheckRequest struct {
	Source  Source  `json:"source"`
	Version Version `json:"version"`
}

type CheckResponse []Version

func (r CheckResponse) Len() int {
	return len(r)
}

func (r CheckResponse) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r CheckResponse) Less(i, j int) bool {
	return r[j].CommittedDate.After(r[i].CommittedDate)
}

func Check(request CheckRequest, manager Gitea) (CheckResponse, error) {
	var response CheckResponse

	// Get pull requests
	prs, err := manager.ListPullRequests(request.Source.State)
	if err != nil {
		return nil, err
	}

	DisableCISkip := request.Source.DisableCISkip

Loop:
	for _, pr := range prs {
		if !DisableCISkip && (ContainsSkipCI(pr.Title) || ContainsSkipCI(pr.Tip.RepoCommit.Message)) {
			continue
		}

		if request.Source.BaseBranch != "" && pr.Base.Name != request.Source.BaseBranch {
			continue
		}

		if !pr.UpdatedDate().After(request.Version.CommittedDate) {
			continue
		}

		if len(request.Source.Labels) > 0 {
			labelFound := false
		LabelLoop:
			for _, targetLabel := range request.Source.Labels {
				for _, prLabel := range pr.Labels {
					if prLabel.Name == targetLabel {
						labelFound = true
						break LabelLoop
					}
				}
			}

			if !labelFound {
				continue Loop
			}
		}

		response = append(response, NewVersion(pr))
	}

	sort.Sort(response)

	// If there are no new but an old version = return the old
	if len(response) == 0 && request.Version.PR != "" {
		response = append(response, request.Version)
	}
	// If there are new versions and no previous = return just the latest
	if len(response) != 0 && request.Version.PR == "" {
		response = CheckResponse{response[len(response)-1]}
	}
	return response, nil
}

// ContainsSkipCI returns true if a string contains [ci skip] or [skip ci] or [no ci].
func ContainsSkipCI(s string) bool {
	re := regexp.MustCompile("(?i)\\[(ci skip|skip ci|no ci)\\]")
	return re.MatchString(s)
}
