---
title: Reducing image size and speeding up a build by mounts
sidebar: documentation
permalink: documentation/configuration/stapel_image/mount_directive.html
author: Artem Kladov <artem.kladov@flant.com>, Alexey Igrychev <alexey.igrychev@flant.com>
summary: |
  <a class="google-drawings" href="https://docs.google.com/drawings/d/e/2PACX-1vReDSY8s7mMtxuxwDTwtPLFYjEXePaoIB-XbEZcunJGNEHrLbrb9aFxyOoj_WeQe0XKQVhq7RWnG3Eq/pub?w=2031&amp;h=144" data-featherlight="image">
      <img src="https://docs.google.com/drawings/d/e/2PACX-1vReDSY8s7mMtxuxwDTwtPLFYjEXePaoIB-XbEZcunJGNEHrLbrb9aFxyOoj_WeQe0XKQVhq7RWnG3Eq/pub?w=1016&amp;h=72">
  </a>

  <div class="language-yaml highlighter-rouge"><pre class="highlight"><code><span class="s">mount</span><span class="pi">:</span>
  <span class="pi">-</span> <span class="s">from</span><span class="pi">:</span> <span class="s">tmp_dir</span>
    <span class="s">to</span><span class="pi">:</span> <span class="s">&lt;absolute_path&gt;</span>
  <span class="pi">-</span> <span class="s">from</span><span class="pi">:</span> <span class="s">build_dir</span>
    <span class="s">to</span><span class="pi">:</span> <span class="s">&lt;absolute_path&gt;</span>
  <span class="pi">-</span> <span class="s">fromPath</span><span class="pi">:</span> <span class="s">&lt;absolute_or_relative_path&gt;</span>
    <span class="s">to</span><span class="pi">:</span> <span class="s">&lt;absolute_path&gt;</span></code></pre>
  </div>
---

Quite often, when building an image, you are left with auxiliary files that serve no purpose and should be excluded from the image. For example:
- Most package managers create a system-wide cache of packages and other files.
  - [APT](https://wiki.debian.org/Apt) saves the package list in the `/var/lib/apt/lists/` directory.
  - APT also saves packages in the `/var/cache/apt/` directory while installing them.
  - [YUM](http://yum.baseurl.org/) keeps downloaded packages in the `/var/cache/yum/.../packages/` directory.
- Package managers for programming languages like ​npm (nodejs), glide (go), pip (python) store files in their respective cache folders.
- The compilation of C, C++, and similar applications leaves behind object files and other files used by the compilers.

Thus, these files:
- serve no purpose in the image;
- could significantly increase the size of an image;
- might be useful in the next image builds.

You can reduce image size and speed up the build process by mounting external folders into assembly containers. Docker implements a mounting mechanism using [volumes](https://docs.docker.com/storage/volumes/).

`mount` config directive is used to specify volumes. Host and assembly container mount points determine each volume (the same goes for `from`/`fromPath` and `to` directives).
When specifying the mount point on the host, you can choose an arbitrary file or a folder defined in `fromPath`, as well as one of the service folders defined in `from`:
- `tmp_dir` is an individual temporary image directory, renewed for each build;
- `build_dir` is a collectively shared directory that is preserved between builds (`~/.werf/shared_context/mounts/projects/<project name>/<mount id>/`).
The various images of the project can use this shared directory to share and store assembly data (e.g., cache).

> werf mounts directories on the host with read/write permissions at each build.
To keep data from these directories in an image, you should copy it to another directory during the build

At `from` stage, werf adds definitions of mount points to labels of a stage image.
Later, each stage uses these definitions to add volumes to an assembly container.
Such an implementation makes it possible to inherit mount points from a [base image]({{ site.baseurl }}/documentation/configuration/stapel_image/base_image.html).

Also, at `from` stage, werf purges mount points of an assembly container in a [base image]({{ site.baseurl }}/documentation/configuration/stapel_image/base_image.html).
Therefore, these folders become empty in an image.
