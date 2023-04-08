## Github PR resource
[original-resource]: https://github.com/telia-oss/github-pr-resource

A Concourse resource for pull requests on Gitea. A port of [the original for Github GraphQL API][original-resource], ported to work with Gitea's RESTful API. 

## Source Configuration

| Parameter                   | Required | Example                          | Description                                                                                                                                                                                                                                                                                |
|-----------------------------|----------|----------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `repository`                | Yes      | `atte/e2e-test-repository`       | The repository to target.                                                                                                                                                                                                                                                                  |
| `access_token`              | Yes      |                                  | A Gitea Access Token with repository access (required for setting status on commits). N.B. If you want github-pr-resource to work with a private repository. Set `repo:full` permissions on the access token you create on GitHub. If it is a public repository, `repo:status` is enough. |
| `endpoint`               | Yes       | `https://git.atte.cloud`         | Endpoint to use for the Gitea API.(Restful).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| `disable_ci_skip`           | No       | `true`                           | Disable ability to skip builds with `[ci skip]`, `[skip ci]` and `[no ci]` in commit message or pull request title.                                                                                                                                                                                   |
| `base_branch`               | No       | `master`                         | Name of a branch. The pipeline will only trigger on pull requests against the specified branch.                                                                                                                                                                                            |
| `labels`                    | No       | `["bug", "enhancement"]`         | The labels on the PR. The pipeline will only trigger on pull requests having at least one of the specified labels.                                                                                                                                                                         |
| `states`                    | No       | `closed`             | The PR states to select (`open`, `closed` or `all`). The pipeline will only trigger on pull requests matching one of the specified states. Default is `open`.                                                                                                                         |

Notes:
 - Look at the [Concourse Resources documentation](https://concourse-ci.org/resources.html#resource-webhook-token)
 for webhook token configuration.

## Behaviour

#### `check`

Produces new versions for all commits (after the last version) ordered by the committed date.
A version is represented as follows:

- `pr`: The pull request number.
- `commit`: The commit SHA.
- `committed`: Timestamp of when the commit was committed. Used to filter subsequent checks.

If several commits are pushed to a given PR at the same time, the last commit will be the new version.

#### `get`

| Parameter            | Required | Example  | Description                                                                        |
|----------------------|----------|----------|------------------------------------------------------------------------------------|
| `skip_download`      | No       | `true`   | Use with `get_params` in a `put` step to do nothing on the implicit get.           |
| `integration_tool`   | No       | `rebase` | The integration tool to use, `merge`, `rebase` or `checkout`. Defaults to `merge`. |
| `git_depth`          | No       | `1`      | Shallow clone the repository using the `--depth` Git option                        |
| `submodules`       | No       | `true` | Recursively clone git submodules. Defaults to false.                        |
| `fetch_tags`       | No       | `true`     | Fetch tags from remote repository                                                  |

Clones the base (e.g. `master` branch) at the latest commit, and merges the pull request at the specified commit
into master. This ensures that we are both testing and setting status on the exact commit that was requested in
input. Because the base of the PR is not locked to a specific commit in versions emitted from `check`, a fresh
`get` will always use the latest commit in master and *report the SHA of said commit in the metadata*. Both the
requested version and the metadata emitted by `get` are available to your tasks as JSON:
- `.git/resource/version.json`
- `.git/resource/metadata.json`

The information in `metadata.json` is also available as individual files in the `.git/resource` directory, e.g. the `base_sha`
is available as `.git/resource/base_sha`.

When specifying `skip_download` the pull request volume mounted to subsequent tasks will be empty, which is a problem
when you set e.g. the pending status before running the actual tests. The workaround for this is to use an alias for
the `put` (see https://github.com/telia-oss/github-pr-resource/issues/32 for more details).
Example here:

```yaml
put: update-status <-- Use an alias for the pull-request resource
resource: pull-request
params:
    path: pull-request
    status: pending
get_params: {skip_download: true}
```

Note that, should you retrigger a build in the hopes of testing the last commit to a PR against a newer version of
the base, Concourse will reuse the volume (i.e. not trigger a new `get`) if it still exists, which can produce
unexpected results (#5). As such, re-testing a PR against a newer version of the base is best done by *pushing an
empty commit to the PR*.

#### `put`

| Parameter                  | Required | Example                              | Description                                                                                                                                                   |
|----------------------------|----------|--------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `path`                     | Yes      | `pull-request`                       | The name given to the resource in a GET step.                                                                                                                 |
| `status`                   | No       | `SUCCESS`                            | Set a status on a commit. One of `pending`, `success`, `error`, `failure`, and `warning`.                                                                                 |
| `base_context`             | No       | `concourse-ci`                       | Base context (prefix) used for the status context. Defaults to `concourse-ci`.                                                                                |
| `context`                  | No       | `unit-test`                          | A context to use for the status, which is prefixed by `base_context`. Defaults to `status`.                                                                   |
| `comment`                  | No       | `hello world!`                       | A comment to add to the pull request.                                                                                                                         |
| `comment_file`             | No       | `my-output/comment.txt`              | Path to file containing a comment to add to the pull request (e.g. output of `terraform plan`).                                                               |
| `target_url`               | No       | `$ATC_EXTERNAL_URL/builds/$BUILD_ID` | The target URL for the status, where users are sent when clicking details (defaults to the Concourse build page).                                             |
| `description`              | No       | `Concourse CI build failed`          | The description status on the specified pull request.                                                                                                         |
| `description_file`         | No       | `my-output/description.txt`          | Path to file containing the description status to add to the pull request                                                                                     |

Note that `comment`, `comment_file` and `target_url` will all expand environment variables, so in the examples above `$ATC_EXTERNAL_URL` will be replaced by the public URL of the Concourse ATCs.
See https://concourse-ci.org/implementing-resource-types.html#resource-metadata for more details about metadata that is available via environment variables.

## Example

```yaml
resource_types:
- name: pull-request
  type: registry-image
  source:
    repository: atteniemi/gitea-pr-resource
    tag: latest

resources:
- name: pull-request
  type: pull-request
  #webhook_token: ((webhook-token))
  source:
    repository: ops/blog
    endpoint: https://git.atte.cloud 
    access_token: ((gitea-access.token))
    state: open

jobs:
- name: test
  plan: 
  - get: pull-request
    trigger: true
    version: every
    params:
      submodules: true
  - put: pull-request
    no_get: true # skip the implied get to not wipe out submodules
    params:
      path: pull-request
      status: pending
  - task: test-build
    config:
      platform: linux
      image_resource:
        type: registry-image
        source:
          repository: ghcr.io/getzola/zola
          tag: v0.17.1
      inputs:
      - name: pull-request
      run:
        path: zola
        args: [-r, pull-request, build]
    on_failure:
      put: pull-request
      params:
        path: pull-request
        status: failure
  - put: pull-request
    no_get: true  # skip the implied get
    params:
      path: pull-request
      status: success
```