package resource

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

func Get(request GetRequest, gitea Gitea, git Git, outputDir string) (*GetResponse, error) {
	if request.Params.SkipDownload {
		return &GetResponse{Version: request.Version}, nil
	}
	pr, err := gitea.GetPullRequest(request.Version.PR, request.Version.Commit)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve pull request: %s", err)
	}
	// Initialize and pull the base for the PR
	if err := git.Init(pr.Base.Ref); err != nil {
		return nil, err
	}
	if err := git.Pull(pr.Base.Repository.CloneURL, pr.Base.Ref, request.Params.GitDepth, request.Params.Submodules, request.Params.FetchTags); err != nil {
		return nil, err
	}

	// Get the last commit SHA in base for the metadata
	baseSHA, err := git.RevParse(pr.Base.Ref)
	if err != nil {
		return nil, err
	}

	// Fetch the PR and merge the specified commit into the base
	//fmt.Printf("%v %v %v %v", pr.Head.Repository.CloneURL, int(pr.Index), request.Params.GitDepth, request.Params.Submodules)
	if err := git.Fetch(pr.Head.Repository.CloneURL, int(pr.Index), request.Params.GitDepth, request.Params.Submodules); err != nil {
		return nil, err
	}

	// Create the metadata
	var metadata Metadata
	metadata.Add("pr", strconv.FormatInt(pr.Index, 10))
	metadata.Add("title", pr.Title)
	metadata.Add("url", pr.URL)
	metadata.Add("head_name", pr.Head.Ref)
	metadata.Add("head_sha", pr.Tip.SHA)
	metadata.Add("base_name", pr.Base.Ref)
	metadata.Add("base_sha", baseSHA)
	metadata.Add("message", pr.Tip.RepoCommit.Message)
	metadata.Add("author", pr.Tip.RepoCommit.Author.Name) // pr.Tip.Author is nil if committer not matched to a Gitea user
	metadata.Add("author_email", pr.Tip.RepoCommit.Author.Email)
	metadata.Add("state", string(pr.State))

	// Write version and metadata for reuse in PUT
	path := filepath.Join(outputDir, ".git", "resource")
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %s", err)
	}
	b, err := json.Marshal(request.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal version: %s", err)
	}
	if err := ioutil.WriteFile(filepath.Join(path, "version.json"), b, 0644); err != nil {
		return nil, fmt.Errorf("failed to write version: %s", err)
	}
	b, err = json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %s", err)
	}
	if err := ioutil.WriteFile(filepath.Join(path, "metadata.json"), b, 0644); err != nil {
		return nil, fmt.Errorf("failed to write metadata: %s", err)
	}

	for _, d := range metadata {
		filename := d.Name
		content := []byte(d.Value)
		if err := ioutil.WriteFile(filepath.Join(path, filename), content, 0644); err != nil {
			return nil, fmt.Errorf("failed to write metadata file %s: %s", filename, err)
		}
	}

	switch tool := request.Params.IntegrationTool; tool {
	case "rebase":
		if err := git.Rebase(pr.Base.Ref, pr.Tip.SHA, request.Params.Submodules); err != nil {
			return nil, err
		}
	case "merge", "":
		if err := git.Merge(pr.Tip.SHA, request.Params.Submodules); err != nil {
			return nil, err
		}
	case "checkout":
		if err := git.Checkout(pr.Head.Ref, pr.Tip.SHA, request.Params.Submodules); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid integration tool specified: %s", tool)
	}

	return &GetResponse{
		Version:  request.Version,
		Metadata: metadata,
	}, nil
}

// GetParameters ...
type GetParameters struct {
	SkipDownload    bool   `json:"skip_download"`
	IntegrationTool string `json:"integration_tool"`
	GitDepth        int    `json:"git_depth"`
	Submodules      bool   `json:"submodules"`
	FetchTags       bool   `json:"fetch_tags"`
}

// GetRequest ...
type GetRequest struct {
	Source  Source        `json:"source"`
	Version Version       `json:"version"`
	Params  GetParameters `json:"params"`
}

// GetResponse ...
type GetResponse struct {
	Version  Version  `json:"version"`
	Metadata Metadata `json:"metadata,omitempty"`
}
