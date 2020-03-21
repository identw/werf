---
title: Adding source code from git repositories
sidebar: documentation
permalink: documentation/configuration/stapel_image/git_directive.html
summary: |
  <a class="google-drawings" href="https://docs.google.com/drawings/d/e/2PACX-1vRUYmRNmeuP14OcChoeGzX_4soCdXx7ZPgNqm5ePcz9L_ItMUqyolRoJyPL7baMNoY7P6M0B08eMtsb/pub?w=2031&amp;h=144" data-featherlight="image">
      <img src="https://docs.google.com/drawings/d/e/2PACX-1vRUYmRNmeuP14OcChoeGzX_4soCdXx7ZPgNqm5ePcz9L_ItMUqyolRoJyPL7baMNoY7P6M0B08eMtsb/pub?w=1016&amp;h=72">
  </a>

  <div class="tabs">
    <a href="javascript:void(0)" class="tabs__btn active" onclick="openTab(event, 'tabs__btn', 'tabs__content', 'local')">Local</a>
    <a href="javascript:void(0)" class="tabs__btn" onclick="openTab(event, 'tabs__btn', 'tabs__content', 'remote')">Remote</a>
  </div>

  <div id="local" class="tabs__content active">
  <div class="language-yaml highlighter-rouge"><div class="highlight"><pre class="highlight"><code><span class="na">git</span><span class="pi">:</span>
  <span class="pi">-</span> <span class="na">add</span><span class="pi">:</span> <span class="s">&lt;absolute path in git repository&gt;</span>
    <span class="na">to</span><span class="pi">:</span> <span class="s">&lt;absolute path inside image&gt;</span>
    <span class="na">owner</span><span class="pi">:</span> <span class="s">&lt;owner&gt;</span>
    <span class="na">group</span><span class="pi">:</span> <span class="s">&lt;group&gt;</span>
    <span class="na">includePaths</span><span class="pi">:</span>
    <span class="pi">-</span> <span class="s">&lt;path or glob relative to path in add&gt;</span>
    <span class="na">excludePaths</span><span class="pi">:</span>
    <span class="pi">-</span> <span class="s">&lt;path or glob relative to path in add&gt;</span>
    <span class="na">stageDependencies</span><span class="pi">:</span>
      <span class="na">install</span><span class="pi">:</span>
      <span class="pi">-</span> <span class="s">&lt;path or glob relative to path in add&gt;</span>
      <span class="na">beforeSetup</span><span class="pi">:</span>
      <span class="pi">-</span> <span class="s">&lt;path or glob relative to path in add&gt;</span>
      <span class="na">setup</span><span class="pi">:</span>
      <span class="pi">-</span> <span class="s">&lt;path or glob relative to path in add&gt;</span></code></pre>
  </div></div>     
  </div>

  <div id="remote" class="tabs__content">
  <div class="language-yaml highlighter-rouge"><div class="highlight"><pre class="highlight"><code><span class="na">git</span><span class="pi">:</span>
  <span class="pi">-</span> <span class="na">url</span><span class="pi">:</span> <span class="s">&lt;git repo url&gt;</span>
    <span class="na">branch</span><span class="pi">:</span> <span class="s">&lt;branch name&gt;</span>
    <span class="na">commit</span><span class="pi">:</span> <span class="s">&lt;commit&gt;</span>
    <span class="na">tag</span><span class="pi">:</span> <span class="s">&lt;tag&gt;</span>
    <span class="na">add</span><span class="pi">:</span> <span class="s">&lt;absolute path in git repository&gt;</span>
    <span class="na">to</span><span class="pi">:</span> <span class="s">&lt;absolute path inside image&gt;</span>
    <span class="na">owner</span><span class="pi">:</span> <span class="s">&lt;owner&gt;</span>
    <span class="na">group</span><span class="pi">:</span> <span class="s">&lt;group&gt;</span>
    <span class="na">includePaths</span><span class="pi">:</span>
    <span class="pi">-</span> <span class="s">&lt;path or glob relative to path in add&gt;</span>
    <span class="na">excludePaths</span><span class="pi">:</span>
    <span class="pi">-</span> <span class="s">&lt;path or glob relative to path in add&gt;</span>
    <span class="na">stageDependencies</span><span class="pi">:</span>
      <span class="na">install</span><span class="pi">:</span>
      <span class="pi">-</span> <span class="s">&lt;path or glob relative to path in add&gt;</span>
      <span class="na">beforeSetup</span><span class="pi">:</span>
      <span class="pi">-</span> <span class="s">&lt;path or glob relative to path in add&gt;</span>
      <span class="na">setup</span><span class="pi">:</span>
      <span class="pi">-</span> <span class="s">&lt;path or glob relative to path in add&gt;</span>
  </code></pre>
  </div></div>
  </div>
---

## What is git mapping?

***Git mapping*** describes a file or a directory from the git repository that should be added to the image by a specific path. The repository may be a local one, hosted in the directory that contains the config, or a remote one, and in this case, the configuration of the _git mapping_ contains address of the repository and version of the code (branch, tag or commit hash).

werf copies files from the repository to the image using the full transfer of files via git archive or by applying patches between commits.
The full transfer is used for the initial addition of files. For subsequent builds, werf applies patches to reflect changes in a git repository. The algorithm behind the full transfer and patching is described in detail in the [More details: git_archive...](#more-details-gitarchive-gitcache-gitlatestpatch) section.

The configuration of the _git mapping_ supports file filtering. You can use a set of _git mappings_ to create virtually any file structure in the image. Also, you can specify owner and group properties of files in the _git mapping_ configuration — no subsequent `chown` is required.

werf supports git submodules. If werf detects that some part of _git mapping_ is a submodule, it does its best to handle the changes in submodules correctly.

> All submodules of a project are associated with a specific commit, so all collaborators working with the submodule repository receive the same content. Thus, werf **does not initialize or update submodules** but just uses the corresponding commits

Here is an example of a _git mapping_ configuration. It allows you to add source files from the `/src` directory in a local repository to the `/app` directory, and remote phantomjs source files to `/src/phantomjs`:

```yaml
git:
- add: /src
  to: /app
- url: https://github.com/ariya/phantomjs
  add: /
  to: /src/phantomjs
```

## Why use git mappings?

The main idea is to bring git history into the build process.

### Patching instead of copying

Most commits in the real application repository are about updating the code of the application itself. In this case, if the compilation is not required, assembling a new image boils down to applying patches to the files in the previous one.

### Remote repositories

Building an application image may depend on source files in other repositories. werf can add files from remote repositories as well as to detect changes in local and remote repositories.

## Syntax of a git mapping

The _git mapping_ configuration for a local repository has the following parameters:

- `add` — the path to a directory or a file whose contents are to be copied to the image. The path relates to the repository root and is absolute (i.e., it must start with `/`). This parameter is optional, contents of the entire repository are transferred by default, i.e., an empty `add` equals to `add: /`;
- `to` — the path in the image where the content specified with `add` will be copied;
- `owner` — the name or uid of the owner of the copied files;
- `group` — the name or gid of the group of the owner;
- `excludePaths` — a set of masks to exlude files or directories during recursive copying. Paths in masks are specified relative to add;
- `includePaths` — a set of masks to include files or directories during recursive copying. Paths in masks are specified relative to add;
- `stageDependencies` — a set of file and folder masks to define the dependence of user-stage rebuilds on their changes. The detailed description available in the [Running assembly instructions]({{ site.baseurl }}/documentation/configuration/stapel_image/assembly_instructions.html).

The _git mapping_ configuration for a remote repository has some additional parameters:
- `url` — remote repository address;
- `branch`, `tag`, `commit` — name of a branch, tag or commit hash that will be used. If these parameters are not specified, the master branch is used.

## Using git mappings

### Copying directories

The `add` parameter specifies the path in a repository, starting with which all files must be recursively retrieved and added to the image at the `to` path; if the parameter is not specified, then the default path (`/`) is used, i.e., the entire repository is transferred.
For example:

```yaml
git:
- add: /
  to: /app
```

This is the basic _git mapping_ configuration that adds the entire contents of the repository to the `/app` directory in the image.

<div class="tabs">
  <a href="javascript:void(0)" class="tabs__btn btn__example1 active" onclick="openTab(event, 'btn__example1', 'tab__example1', 'git-mapping-01-source')">Structure of a Git repo</a>
  <a href="javascript:void(0)" class="tabs__btn btn__example1" onclick="openTab(event, 'btn__example1', 'tab__example1', 'git-mapping-01-dest')">Structure of the resulting image</a>
</div>
<div id="git-mapping-01-source" class="tabs__content tab__example1 active">
  <img src="{{ site.baseurl }}/images/build/git_mapping_01.png" alt="git repository files tree" />
</div>
<div id="git-mapping-01-dest" class="tabs__content tab__example1">
  <img src="{{ site.baseurl }}/images/build/git_mapping_02.png" alt="image files tree" />
</div>

You can specify multiple _git mappings_:

```yaml
git:
- add: /src
  to: /app/src
- add: /assets
  to: /static
```

<div class="tabs">
  <a href="javascript:void(0)" class="tabs__btn btn__example2 active" onclick="openTab(event, 'btn__example2', 'tab__example2', 'git-mapping-02-source')">Structure of a Git repo</a>
  <a href="javascript:void(0)" class="tabs__btn btn__example2" onclick="openTab(event, 'btn__example2', 'tab__example2', 'git-mapping-02-dest')">Structure of the resulting image</a>
</div>
<div id="git-mapping-02-source" class="tabs__content tab__example2 active">
  <img src="{{ site.baseurl }}/images/build/git_mapping_03.png" alt="git repository files tree" />
</div>
<div id="git-mapping-02-dest" class="tabs__content tab__example2">
  <img src="{{ site.baseurl }}/images/build/git_mapping_04.png" alt="image files tree" />
</div>


It should be noted that _git mapping_ configuration doesn't specify the directory to be transferred (like `cp -r /src /app`). The `add` parameter specifies the contents of a directory that will be recursively copied from the repository. That is, if the `/assets` directory needs to be transferred to the `/app/assets` directory, then the name **assets** should be written twice, or, as an option, you can use an `includePaths` [filter](#using-filters).

```yaml
git:
- add: /assets
  to: /app/assets
```

or

```yaml
git:
- add: /
  to: /app
  includePaths: assets
```

> werf has no convention for trailing `/` that is available in rsync, i.e., `add: /src` and `add: /src/` are the same

### Changing an owner

The _git mapping_ configuration provides `owner` and `group` parameters. These are the names or numerical ids of the owner and group used for all files and directories transferred to the image.

```yaml
git:
- add: /src/index.php
  to: /app/index.php
  owner: www-data
```

![index.php owned by www-data user and group]({{ site.baseurl }}/images/build/git_mapping_05.png)

If only the `owner` parameter is specified, then the group for files is presumed to be the same as the primary group of the specified user.

If an `owner` or a `group` parameter has a string value, then the specified user or a group must exist in the system before the transfer of files is complete (you have to add them in advance if necessary, e.g., at the beforeInstall stage), otherwise, an error will occur during the build process.

```yaml
git:
- add: /src/index.php
  to: /app/index.php
  owner: wwwdata
```



### Using filters

The`includePaths` and `excludePaths` parameters are used for composing the list of files to add to the image. These parameters include sets of file masks. You can use them to include/exclude files and directories to/from the list of files, that will be added to the image. The `excludePaths` filter works as follows: file masks are applied to each file found in the `add` path. If there is at least one match, then the file is ignored; if no matches are found, then the file is added to the image. The `includePaths` works the opposite way: if there is at least one match, the file is added to the image.

The _git mapping_ configuration may contain both filters. In this case, a file is added to the image if its path matches one of `includePaths` masks and does not match any of `excludePaths` masks.

For example:

```yaml
git:
- add: /src
  to: /app
  includePaths:
  - '**/*.php'
  - '**/*.js'
  excludePaths:
  - '**/*-dev.*'
  - '**/*-test.*'
```

The above _git mapping_ configuration adds `.php` and `.js` files from the `/src` path except for files with `-dev.` or `-test.` suffixes.

When determining whether a file matches a mask, the following algorithm is used:
 - determine the absolute path to the file in the repository;
 - compare it with the masks defined in includePaths/excludePaths or the specific path:
   - the path in `add` is concatenated with the mask or raw path from include or exclude config directive;
   - two paths are compared using glob patterns: if the file matches the mask, then it will be included (for `includePaths`) or excluded (for `excludePaths`), the algorithm is ended.
 - compare this path with configured include/exclude path mask or the specific path with an additional pattern:
   - the path in `add` is concatenated with the mask or raw path from include or exclude config directive and concatenated with additional suffix pattern `**/*`;
   - two paths are compared with the use of glob patterns: if the file matches the mask, then it will be included (for `includePaths`) or excluded (for `excludePaths`); the algorithm is complete.

> The step that involves the addition of the `**/*` pattern is here just for convenience: the common use case of a _git mapping_ with filters involves setting up recursive copying of the directory. Thanks to adding the `**/*` pattern, you can specify just the name of a directory so that its entire contents would match the filter

Mask may contain the following patterns:

- `*` — matches any file. This pattern includes `.` and exclude `/`
- `**` — matches directories recursively or files expansively
- `?` — matches any single character. Equivalent to /.{1}/ in regexp
- `[set]` — matches any single character in the set. This pattern behaves exactly like character sets in regexp, including set negation ([^a-z])
- `\` — escapes the next metacharacter

Mask that starts with `*` or `**` patterns should be escaped with quotes in the `werf.yaml` file:
 - `"*.rb"` — with double quotes
- `'**/*'` — with single quotes

Examples of filters:

```yaml
add: /src
to: /app
includePaths:
# match all php files residing directly in /src
- '*.php'

# matches recursively all php files from /src
# (also matches *.php because '.' is included in **)
- '**/*.php'

# matches all files from /src/module1 recursively
# an example of implicit adding of **/*
- module1
```

The `includePaths` filter can be used to copy any file without renaming:
```yaml
git:
- add: /src
  to: /app
  includePaths: index.php
```

### Target paths overlapping

It is important to note that if there are multiple _git mappings_, the paths defined in the `to` field may interfere with each other, resulting in the inability to add files to the image. For example:

```yaml
git:
- add: /src
  to: /app
- add: /assets
  to: /app/assets
```

When processing a config, werf finds all the possible intersections among _git mappings_ concerning `includePaths` and `excludePaths` filters. If an intersection is detected, werf tries to resolve uncomplicated conflicts by implicitly adding `excludePaths` into the _git mapping_. In complicated cases, the build ends with an error. Please note that the implicit `excludePaths` filter can have undesirable effects, so try to avoid conflicts of intersecting paths between git mappings.

Below is an example of implicit `excludePaths`:

```yaml
git:
- add: /src
  to: /app
  excludePaths:  # werf add this filter to resolve a conflict
  - assets       # between paths /src/assets and /assets
- add: /assets
  to: /app/assets
```

## Working with remote repositories

werf can use remote repositories as a source of files. For this purpose, the _git mapping_ configuration has an `url` field with the repository address. werf supports `https` and `git+ssh` protocols.

### https

The syntax for the https protocol is as follows:

{% raw %}
```yaml
git:
- url: https://[USERNAME[:PASSWORD]@]repo_host/repo_path[.git/]
```
{% endraw %}

You may need a login and password to access over `https`.

Here is an example of accessing the repository from the GitLab CI pipeline using environmental variables:

{% raw %}
```yaml
git:
- url: https://{{ env "CI_REGISTRY_USER" }}:{{ env "CI_JOB_TOKEN" }}@registry.gitlab.company.name/common/helper-utils.git
```
{% endraw %}

In this example, the [env](http://masterminds.github.io/sprig/os.html) method from the sprig library is used to access the environment variables.

### git, ssh

werf supports access to the repository via the git protocol. Access via this protocol is typically protected using ssh tools: this feature is used by GitHub, Bitbucket, GitLab, Gogs, Gitolite, etc. Most often the repository address looks as follows:

```yaml
git:
- url: git@gitlab.company.name:project_group/project.git
```

To successfully work with remote repositories via ssh, you should understand how werf searches for access keys.


#### Working with ssh keys

Keys for ssh connects are provided by ssh-agent. The ssh-agent is a daemon that operates via file socket, the path to which is stored in the environment variable `SSH_AUTH_SOCK`. werf mounts this file socket to all _assembly containers_ and sets the environment variable `SSH_AUTH_SOCK`, i.e., connection to remote git repositories is established with the use of keys that are registered in the running ssh-agent.

The ssh-agent is determined as follows:

- If werf is started with `--ssh-key` flags (there may be multiple flags):
  - A temporary ssh-agent runs with defined keys, and it is used for all git operations with remote repositories.
  - The already running ssh-agent is ignored in this case.
- No `--ssh-key` flags specified and ssh-agent is running:
  - `SSH_AUTH_SOCK` environment variable is used, and the keys added to this agent is used for git operations.
- No `--ssh-key` flags specified and ssh-agent is not running:
  - If `~/.ssh/id_rsa` file exists, then werf will run the temporary ssh-agent with the  key from `~/.ssh/id_rsa` file.
- If none of the previous options is applicable, then the ssh-agent is not started, and no keys for git operation are available. Build images with remote _git mappings_ ends with an error.

## More details: gitArchive, gitCache, gitLatestPatch

Let us review adding files to the resulting image in more detail. As stated earlier, the docker image contains multiple layers. To understand what layers werf create, let's consider the building actions based on three sample commits: `1`, `2` and `3`:

- Build of commit No. 1. All files are added to a single layer based on the configuration of the _git mappings_. This is done with the help of the git archive. This is the layer of the _gitArchive_ stage.
- Build of commit No. 2. Another layer is added where the files are changed by applying a patch. This is the layer of the _gitLatestPatch_ stage.
- Build of commit No. 3. Files have already added, so werf apply patches in the _gitLatestPatch_ stage layer.

Build sequence for these commits may be represented as follows:

| | gitArchive | --- | gitLatestPatch |
|---|:---:|:---:|:---:|
| Commit No. 1 is made, build at 10:00 |  files as in commit No. 1 | --- | - |
| Commit No. 2 is made, build at 10:05 |  files as in commit No. 1 | --- | files as in commit No. 2 |
| Commit No. 3 is made, build at 10:15 |  files as in commit No. 1 | --- | files as in commit No. 3 |

A space between the layers in this table is not accidental. After a while, the number of commits grows, and the patch between commit No. 1 and the current commit may become quite large, which will further increase the size of the last layer and the total _stages_ size. To prevent the growth of the last layer werf provides another intermediary stage — _gitCache_.
How does werf work with these three stages? Now we are going to need more commits to illustrate this, let it be `1`, `2`, `3`, `4`, `5`, `6` and `7`.

- Build of commit No. 1. As before, files are added to a single layer based on the configuration of the _git mappings_. This is done with the help of the git archive. This is the layer of the _gitArchive_ stage.
- Build of commit No. 2. The size of the patch between `1` and `2` does not exceed 1 MiB, so only the layer of the _gitLatestPatch_ stage is modified by applying the patch between `1` and `2`.
- Build of commit No. 3. The size of the patch between `1` and `3` does not exceed 1 MiB, so only the layer of the _gitLatestPatch_ stage is modified by applying the patch between `1` and `3`.
- Build of commit No. 4. The size of the patch between `1` and `4` exceeds 1 MiB. Now _gitCache_ stage layer is added by applying the patch between `1` and `4`.
- Build of commit No. 5. The size of the patch between `4` and `5` does not exceed 1 MiB, so only the layer of the _gitLatestPatch_ stage is modified by applying the patch between `4` and `5`.

This means that as commits are added starting from the moment the first build is done, big patches are gradually accumulated into the layer for the _gitCache_ stage, and only patches with moderate size are applied in the layer for the last _gitLatestPatch_ stage. This algorithm reduces the size of _stages_.

| | gitArchive | gitCache | gitLatestPatch |
|---|:---:|:---:|:---:|
| Commit No. 1 is made, build at 12:00 |  1 |  - | - |
| Commit No. 2 is made, build at 12:19 |  1 |  - | 2 |
| Commit No. 3 is made, build at 12:25 |  1 |  - | 3 |
| Commit No. 4 is made, build at 12:45 |  1 | *4 | - |
| Commit No. 5 is made, build at 12:57 |  1 |  4 | 5 |

\* — the size of the patch for commit `4` exceeded 1 MiB, so this patch is applied in the layer for the _gitCache_ stage.

### Rebuild of gitArchive stage

For various reasons, you may want to reset the _gitArchive_ stage, for example, to decrease the size of _stages_ and the image.

To illustrate the unnecessary growth of image size assume the rare case of 2GiB file in git repository. First build transfers this file in the layer of the _gitArchive_ stage. Then some optimization occured and file is recompiled and it's size is decreased to 1.6GiB. The build of this new commit applies patch in the layer of the _gitCache_ stage. The image size become 3.6GiB of which 2GiB is a cached old version of the big file. Rebuilding from _gitArchive_ stage can reduce image size to 1.6GiB. This situation is quite rare but gives a good explanation of correlation between the layers of the _git stages_.

You can reset the _gitArchive_ stage specifying the **[werf reset]** or **[reset werf]** string in the commit message. Let us assume that, in the previous example commit `6` contains **[werf reset]** in its message, and then the builds would look as follows:

| | gitArchive | gitCache | gitLatestPatch |
|---|:---:|:---:|:---:|
| Commit No. 1 is made, build at 12:00 |  1 |  - | - |
| Commit No. 2 is made, build at 12:19 |  1 |  - | 2 |
| Commit No. 3 is made, build at 12:25 |  1 |  - | 3 |
| Commit No. 4 is made, build at 12:45 |  1 |  4 | - |
| Commit No. 5 is made, build at 12:57 |  1 |  4 | 5 |
| Commit No. 6 is made, build at 13:22 |  *6 |  - | - |

\* — commit `6` contains the **[werf reset]** string in its message, so the _gitArchive_ stage is rebuilt.

### _git stages_ and rebasing

Each _git stage_ stores service labels with commits SHA from which this _stage_ was built.
These commits are used for creating patches on the next _git stage_ (in a nutshell, `git diff COMMIT_FROM_PREVIOUS_GIT_STAGE LATEST_COMMIT` for each described _git mapping_).
So, if any saved commit is not in a git repository (e.g., after rebasing) then werf rebuilds that stage with latest commits at the next build.
