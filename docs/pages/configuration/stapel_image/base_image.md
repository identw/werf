---
title: Base image
sidebar: documentation
permalink: documentation/configuration/stapel_image/base_image.html
author: Alexey Igrychev <alexey.igrychev@flant.com>
summary: |
  <a class="google-drawings" href="https://docs.google.com/drawings/d/e/2PACX-1vReDSY8s7mMtxuxwDTwtPLFYjEXePaoIB-XbEZcunJGNEHrLbrb9aFxyOoj_WeQe0XKQVhq7RWnG3Eq/pub?w=2031&amp;h=144" data-featherlight="image">
      <img src="https://docs.google.com/drawings/d/e/2PACX-1vReDSY8s7mMtxuxwDTwtPLFYjEXePaoIB-XbEZcunJGNEHrLbrb9aFxyOoj_WeQe0XKQVhq7RWnG3Eq/pub?w=1016&amp;h=72" alt="Base image">
  </a>

  <div class="language-yaml highlighter-rouge"><div class="highlight"><pre class="highlight"><code><span class="na">from</span><span class="pi">:</span> <span class="s">&lt;image[:&lt;tag&gt;]&gt;</span>
  <span class="na">fromLatest</span><span class="pi">:</span> <span class="s">&lt;bool&gt;</span>
  <span class="na">fromCacheVersion</span><span class="pi">:</span> <span class="s">&lt;arbitrary string&gt;</span>
  <span class="na">fromImage</span><span class="pi">:</span> <span class="s">&lt;image name&gt;</span>
  <span class="na">fromImageArtifact</span><span class="pi">:</span> <span class="s">&lt;artifact name&gt;</span>
  </code></pre></div>
  </div>
---

Here's a minimal `werf.yaml`. It describes an `example` _image_ that is based on the _base image_ named `alpine`:

```yaml
project: my-project
configVersion: 1
---
image: example
from: alpine
```

The _base image_ can be declared with `from`, `fromImage` or `fromImageArtifact` directives.

## from, fromLatest

The `from` directive defines the name and tag of the _base image_. If no tag is specified, it defaults to `latest`.

```yaml
from: <image>[:<tag>]
```

By default, the assembly process does not depend on the current _base image_ digest in the repository; it depends on the _from_ directive value only.
Therefore, changing the _base image_ in the local storage or the repository will not affect the build if _from_ stage already exists in the _stages storage_.

If you want to build the image with the actual _base image_, you should use _fromLatest_ directive.
_fromLatest_ directive allows you to connect the assembly process with the current _base image_ digest from the repository.
```yaml
fromLatest: true
```

> Please note that werf uses the current _base image_ digest as an extra dependency for the _from_ stage if _fromLatest_ is set to true. In this case, werf starts using the digest of the current _base image_ when calculating signatures of the _from_ stage.
Therefore, the usage of _fromLatest_ can lead to non-reproducible signatures:
if there are changes in the _base image_ in the repository, then all previously built stages, as well as related images, become non-usable.
Here are examples of problems that this behavior can lead to in CI processes:
- The build is successful, but then the _base image_ gets changed, and subsequent pipeline jobs (e.g., deploy) no longer work. This is because the final image built with the updated _base image_ does not exist yet.
- The built application is successfully deployed, but then the _base image_ gets updated and redeploying no longer works. The reason is the same as in the previous case.

## fromImage and fromImageArtifact

Besides using a docker image stored in a repository, the _base image_ can refer to an _image_ or an [_artifact_]({{ site.baseurl }}/documentation/configuration/stapel_artifact.html) described in the same `werf.yaml` file. In this case, you should use fromImage and fromImageArtifact directives, respectively.

```yaml
fromImage: <image name>
fromImageArtifact: <artifact name>
```

If the _base image_ is specific to a particular application,
it makes sense to store its description along with _images_ and _artifacts_ that use it instead of keeping the _base image_ in a Docker registry.

Also, this method can be useful if the existing _stage conveyor_ doesn't suit your needs for some reason. In this case, you can design your own _stage conveyor_.

<a class="google-drawings" href="https://docs.google.com/drawings/d/e/2PACX-1vTmQBPjB6p_LUpwiae09d_Jp0JoS6koTTbCwKXfBBAYne9KCOx2CvcM6DuD9pnopdeHF--LPpxJJFhB/pub?w=1629&amp;h=1435" data-featherlight="image">
<img src="https://docs.google.com/drawings/d/e/2PACX-1vTmQBPjB6p_LUpwiae09d_Jp0JoS6koTTbCwKXfBBAYne9KCOx2CvcM6DuD9pnopdeHF--LPpxJJFhB/pub?w=850&amp;h=673" alt="Conveyor with fromImage and fromImageArtifact stages">
</a>

## fromCacheVersion

Using the `fromCacheVersion` directive, you can influence the _from_ stage signature (since the value of fromCacheVersion is part of the stage signature) and, therefore, enforce the rebuilding of the image. Any change to `fromCacheVersion` would affect the signature of the _from_ stage (and all subsequent stages, obviously) regardless of whether the _base image_ (or its digest) has changed or remained the same. As a result, all stages will be rebuilt.

```yaml
fromCacheVersion: <arbitrary string>
```
