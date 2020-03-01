---
title: Adding docker instructions
sidebar: documentation
permalink: documentation/configuration/stapel_image/docker_directive.html
author: Alexey Igrychev <alexey.igrychev@flant.com>
summary: |
  <a class="google-drawings" href="https://docs.google.com/drawings/d/e/2PACX-1vTZB0BLxL7mRUFxkrOMaj310CQgb5D5H_V0gXe7QYsTu3kKkdwchg--A1EoEP2CtKbO8pp2qARfeoOK/pub?w=2031&amp;h=144" data-featherlight="image">
    <img src="https://docs.google.com/drawings/d/e/2PACX-1vTZB0BLxL7mRUFxkrOMaj310CQgb5D5H_V0gXe7QYsTu3kKkdwchg--A1EoEP2CtKbO8pp2qARfeoOK/pub?w=1016&amp;h=72">
  </a>

    <div class="language-yaml highlighter-rouge"><div class="highlight"><pre class="highlight"><code><span class="na">docker</span><span class="pi">:</span>
    <span class="na">VOLUME</span><span class="pi">:</span>
    <span class="pi">-</span> <span class="s">&lt;volume&gt;</span>
    <span class="na">EXPOSE</span><span class="pi">:</span>
    <span class="pi">-</span> <span class="s">&lt;expose&gt;</span>
    <span class="na">ENV</span><span class="pi">:</span>
      <span class="s">&lt;env_name&gt;</span><span class="pi">:</span> <span class="s">&lt;env_value&gt;</span>
    <span class="na">LABEL</span><span class="pi">:</span>
      <span class="s">&lt;label_name&gt;</span><span class="pi">:</span> <span class="s">&lt;label_value&gt;</span>
    <span class="na">ENTRYPOINT</span><span class="pi">:</span> <span class="s">&lt;entrypoint&gt;</span>
    <span class="na">CMD</span><span class="pi">:</span> <span class="s">&lt;cmd&gt;</span>
    <span class="na">WORKDIR</span><span class="pi">:</span> <span class="s">&lt;workdir&gt;</span>
    <span class="na">USER</span><span class="pi">:</span> <span class="s">&lt;user&gt;</span>
    <span class="na">HEALTHCHECK</span><span class="pi">:</span> <span class="s">&lt;healthcheck&gt;</span></code></pre></div></div>
---

[Dockerfile instructions](https://docs.docker.com/engine/reference/builder/) can be divided into two groups: build-time instructions and instructions that affect an image manifest. Build-time instructions do not make sense in a werf build process. Since werf builder uses custom syntax to describe the assembly process, only the following Dockerfile instructions from the second group are supported:

* `USER` sets the user and the group to use when running the image (read [more](https://docs.docker.com/engine/reference/builder/#user)).
* `WORKDIR` sets the working directory (read [more](https://docs.docker.com/engine/reference/builder/#workdir)).
* `VOLUME` adds a mount point (read [more](https://docs.docker.com/engine/reference/builder/#volume)).
* `ENV` sets the environment variable (read [more](https://docs.docker.com/engine/reference/builder/#env)).
* `LABEL` adds metadata to an image (read [more](https://docs.docker.com/engine/reference/builder/#label)).
* `EXPOSE` informs Docker that the container listens on the specified network ports at runtime (read [more](https://docs.docker.com/engine/reference/builder/#expose))
* `ENTRYPOINT` helps to configure a container that will run as an executable (read [more](https://docs.docker.com/engine/reference/builder/#entrypoint)).
* `CMD` provides default arguments for the `ENTRYPOINT` to configure a container that will run as an executable (read [more](https://docs.docker.com/engine/reference/builder/#cmd)).
* `HEALTHCHECK` tells Docker how to test a container to check that it is still working (read [more](https://docs.docker.com/engine/reference/builder/#healthcheck))

You can specify the above instructions in the `docker` config directive.

Here is an example of using docker instructions:

```yaml
docker:
  WORKDIR: /app
  CMD: ['python', './index.py']
  EXPOSE: '5000'
  ENV:
    TERM: xterm
    LC_ALL: en_US.UTF-8
```

The specified docker instructions are applied during the last stage called `docker_instructions`.
Therefore, instructions do not affect other stages - they only append data to the assembled image.

If you require some specific environment variables when building your application image (such as a `TERM` environment), then you should use the [base image]({{ site.baseurl }}/documentation/configuration/stapel_image/base_image.html) in which these variables are set.

> Tip: you can also export environment variables at the [_user stage_]({{ site.baseurl }}/documentation/configuration/stapel_image/assembly_instructions.html#what-is-user-stages).
