# The Docker executor

GitLab Runner can use Docker to run jobs on user provided images. This is
possible with the use of **Docker** executor.

The **Docker** executor when used with GitLab CI, connects to [Docker Engine]
and runs each build in a separate and isolated container using the predefined
image that is [set up in `.gitlab-ci.yml`][yaml] and in accordance in
[`config.toml`][toml].

That way you can have a simple and reproducible build environment that can also
run on your workstation. The added benefit is that you can test all the
commands that we will explore later from your shell, rather than having to test
them on a dedicated CI server.

The following table lists what combinations of containers, executors,
and OS are supported.

| Container Type    | Executor         | OS Type | Supported |
|:-----------------:|:----------------:|:-------:|:---------:|
| Windows Container | `docker`         | Windows | ✗         |
| Windows Container | `docker`         | Linux   | ✗         |
| Windows Container | `docker-windows` | Windows | ✓         |
| Windows Container | `docker-windows` | Linux   | ✗         |
| Linux Containers  | `docker`         | Linux   | ✓         |
| Linux Containers  | `docker`         | Windows | ✓         |
| Linux Containers  | `docker-windows` | Linux   | ✗         |
| Linux Containers  | `docker-windows` | Windows | ✗         |

NOTE: **Note:**
GitLab Runner uses Docker Engine API
[v1.25](https://docs.docker.com/engine/api/v1.25/) to talk to the Docker
Engine. This means the
[minimum supported version](https://docs.docker.com/develop/sdk/#api-version-matrix)
of Docker is `1.13.0`.

## Using Windows containers

> [Introduced](https://gitlab.com/groups/gitlab-org/-/epics/535) in 11.11.

To use Windows containers with the Docker executor, note the following
information about limitations, supported Windows versions, and
configuring a Windows Docker executor.

### Limitations

The following are some limitations of using Windows containers with
Docker executor:

- Nanoserver cannot be used because it requires PowerShell 6 but GitLab
  requires PowerShell 5 (see
  [#3291](https://gitlab.com/gitlab-org/gitlab-runner/issues/3291)). See
  also the list of [supported Windows
  versions](#supported-windows-versions).
- Docker-in-Docker is not supported, since it's [not
  supported](https://github.com/docker-library/docker/issues/49) by
  Docker itself.
- Interactive web terminals are not supported.
- Host device mounting not supported.
- When mounting a volume directory it has to exist, or Docker will fail
  to start the container, see
  [#3754](https://gitlab.com/gitlab-org/gitlab-runner/issues/3754) for
  additional detail.
- `docker-windows` executor can be run only using GitLab Runner running
  on Windows.
- [Linux containers on
  Windows](https://docs.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/linux-containers)
  are not supported, since they are still experimental. Read [the
  relevant
  issue](https://gitlab.com/gitlab-org/gitlab-runner/issues/4373) for
  more details.
- Because of a [limitation in
  Docker](https://github.com/MicrosoftDocs/Virtualization-Documentation/issues/334),
  if the destination path drive letter is not `c:`, paths are not supported for:

  - [`builds_dir`](../configuration/advanced-configuration.md#the-runners-section)
  - [`cache_dir`](../configuration/advanced-configuration.md#the-runners-section)
  - [`volumes`](../configuration/advanced-configuration.md#volumes-in-the-runnersdocker-section)

  This means values such as `f:\\cache_dir` are not supported, but `f:` is supported.
  However, if the destination path is on the `c:` drive, paths are also supported
  (for example `c:\\cache_dir`).

### Supported Windows versions

GitLab Runner only supports the following versions of Windows which
follows our [support lifecycle for
Windows](../install/windows.md#windows-version-support-policy):

- Windows Server 1809.
- Windows Server 1803 *Deprecated*.

For future Windows Server versions, we have a [future version support
policy](../install/windows.md#windows-version-support-policy).

You can only run containers based on the same OS version that the Docker
daemon is running on. For example, the following [`Windows Server
Core`](https://hub.docker.com/_/microsoft-windows-servercore) images can
be used:

- `mcr.microsoft.com/windows/servercore:1809`
- `mcr.microsoft.com/windows/servercore:1809-amd64`
- `mcr.microsoft.com/windows/servercore:ltsc2019`
- `mcr.microsoft.com/windows/servercore:1803`

### Configuring a Windows Docker executor

NOTE: **Note:**
There is a known issue when a new Runner is registered with `c:\\cache`
as a source directory when passing the `--docker-volumes` or
`DOCKER_VOLUMES` environment variable. For more details check
[#4312](https://gitlab.com/gitlab-org/gitlab-runner/issues/4312)

Below is an example of what the configuration for a simple Docker
executor running Windows

```toml
[[runners]]
  name = "windows-docker-2019"
  url = "https://gitlab.com/"
  token = "xxxxxxx"
  executor = "docker-windows"
  [runners.docker]
    image = "mcr.microsoft.com/windows/servercore:1809_amd64"
    volumes = ["c:\\cache"]
```

For other configuration options for the Docker executor, see the
[advanced
configuration](../configuration/advanced-configuration.md#the-runnersdocker-section)
section.

### Services

You can use [services](https://docs.gitlab.com/ee/ci/services/) by
enabling [network per-build](#network-per-build) networking mode.
[Available](https://gitlab.com/gitlab-org/gitlab-runner/issues/1042)
since GitLab Runner 12.9.

## Workflow

The Docker executor divides the job into multiple steps:

1. **Prepare**: Create and start the [services](https://docs.gitlab.com/ee/ci/yaml/#services).
1. **Pre-job**: Clone, restore [cache](https://docs.gitlab.com/ee/ci/yaml/#cache)
   and download [artifacts](https://docs.gitlab.com/ee/ci/yaml/#artifacts) from previous
   stages. This is run on a special Docker image.
1. **Job**: User build. This is run on the user-provided Docker image.
1. **Post-job**: Create cache, upload artifacts to GitLab. This is run on
   a special Docker Image.

The special Docker image is based on [Alpine Linux] and contains all the tools
required to run the prepare, pre-job, and post-job steps, like the Git and the
Runner binaries for supporting caching and artifacts. You can find the definition of
this special image [in the official Runner repository][special-build].

## The `image` keyword

The `image` keyword is the name of the Docker image that is present in the
local Docker Engine (list all images with `docker images`) or any image that
can be found at [Docker Hub][hub]. For more information about images and Docker
Hub please read the [Docker Fundamentals][] documentation.

In short, with `image` we refer to the docker image, which will be used to
create a container on which your build will run.

If you don't specify the namespace, Docker implies `library` which includes all
[official images](https://hub.docker.com/u/library/). That's why you'll see
many times the `library` part omitted in `.gitlab-ci.yml` and `config.toml`.
For example you can define an image like `image: ruby:2.6`, which is a shortcut
for `image: library/ruby:2.6`.

Then, for each Docker image there are tags, denoting the version of the image.
These are defined with a colon (`:`) after the image name. For example, for
Ruby you can see the supported tags at <https://hub.docker.com/_/ruby/>. If you
don't specify a tag (like `image: ruby`), `latest` is implied.

NOTE: **Note:**
The image you choose to run your build in via `image` directive must have a
working shell in its operating system `PATH`. Supported shells are `sh` or
`bash` for Linux, `cmd` or PowerShell for Windows. GitLab Runner cannot
execute a command using the underlying OS system calls (like `exec`).

## The `services` keyword

The `services` keyword defines just another Docker image that is run during
your build and is linked to the Docker image that the `image` keyword defines.
This allows you to access the service image during build time.

The service image can run any application, but the most common use case is to
run a database container, e.g., `mysql`. It's easier and faster to use an
existing image and run it as an additional container than install `mysql` every
time the project is built.

You can see some widely used services examples in the relevant documentation of
[CI services examples](https://gitlab.com/gitlab-org/gitlab-ce/tree/master/doc/ci/services/README.md).

If needed, you can [assign an alias](https://docs.gitlab.com/ee/ci/docker/using_docker_images.html#available-settings-for-services)
to each service.

## Networking

Networking is required to connect services to the build job and may also be used to run build jobs in user-defined
networks. Either legacy `network_mode` or `per-build` networking may be used.

### Legacy container links

The default network mode uses [Legacy container links](https://docs.docker.com/network/links/) with
the default Docker `bridge` mode to link the job container with the services.

`network_mode` can be used to configure how the networking stack is set up for the containers
using one of the following values:

- One of the standard Docker [networking modes](https://docs.docker.com/engine/reference/run/#network-settings):
  - `bridge`: use the bridge network (default)
  - `host`: use the host's network stack inside the container
  - `none`: no networking (not recommended)
  - Any other `network_mode` value is taken as the name of an already existing
    Docker network, which the build container should connect to.

For name resolution to work, Docker will manipulate the `/etc/hosts` file in the build
job container to include the service container hostname (and alias). However,
the service container will **not** be able to resolve the build job container
name. To achieve that, use the `per-build` network mode.

NOTE: **Note:**
Linked containers will share their environment variables.

### Network per-build

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/issues/1042) in GitLab Runner 12.9.

This mode will create and use a new user-defined Docker bridge network per build.
[User-defined bridge networks](https://docs.docker.com/network/bridge/) are covered in detail in the Docker documentation.

NOTE: **Note:**
Unlike [legacy container links](#legacy-container-links) used in other network modes,
Docker environment variables will **Not** be shared across the containers.

NOTE: **Note:**
Docker networks may conflict with other networks on the host, including other Docker networks,
if the CIDR ranges are already in use. The default Docker address pool can be configured
via `default-address-pool` in [`dockerd`](https://docs.docker.com/engine/reference/commandline/dockerd/).

To enable this mode you need to enable the [`FF_NETWORK_PER_BUILD`
feature flag](../configuration/feature-flags.md).

When a job starts, a bridge network is created (similarly to `docker
network create <network>`). Upon creation, the service container(s) and the
build job container are connected to this network.

Both the build job container and the service container(s) will be able to
resolve each others' hostnames (and aliases). This functionality is
[provided by Docker](https://docs.docker.com/network/bridge/#differences-between-user-defined-bridges-and-the-default-bridge).

The build container is resolvable via the `build` alias as well as it's GitLab assigned hostname.

The network is removed at the end of the build job.

## Define image and services from `.gitlab-ci.yml`

You can simply define an image that will be used for all jobs and a list of
services that you want to use during build time.

```yaml
image: ruby:2.6

services:
  - postgres:9.3

before_script:
  - bundle install

test:
  script:
  - bundle exec rake spec
```

It is also possible to define different images and services per job:

```yaml
before_script:
  - bundle install

test:2.6:
  image: ruby:2.6
  services:
  - postgres:9.3
  script:
  - bundle exec rake spec

test:2.7:
  image: ruby:2.7
  services:
  - postgres:9.4
  script:
  - bundle exec rake spec
```

## Define image and services in `config.toml`

Look for the `[runners.docker]` section:

```toml
[runners.docker]
  image = "ruby:2.6"

[[runners.docker.services]]
  name = "mysql:latest"
  alias = "db"

[[runners.docker.services]]
  name = "redis:latest"
  alias = "cache"
```

NOTE: **Note:**
The example above uses the [array of tables syntax](https://github.com/toml-lang/toml#user-content-array-of-tables).
Defining services with the [array of strings syntax](https://github.com/toml-lang/toml#user-content-array)
is deprecated and will be removed in a future version.

The image and services defined this way will be added to all builds run by
that Runner, so even if you don't define an `image` inside `.gitlab-ci.yml`,
the one defined in `config.toml` will be used.

## Define an image from a private Docker registry

Starting with GitLab Runner 0.6.0, you are able to define images located to
private registries that could also require authentication.

All you have to do is be explicit on the image definition in `.gitlab-ci.yml`.

```yaml
image: my.registry.tld:5000/namepace/image:tag
```

In the example above, GitLab Runner will look at `my.registry.tld:5000` for the
image `namespace/image:tag`.

If the repository is private you need to authenticate your GitLab Runner in the
registry. Read more on [using a private Docker registry][runner-priv-reg].

## Accessing the services

Let's say that you need a Wordpress instance to test some API integration with
your application.

You can then use for example the [tutum/wordpress](https://hub.docker.com/r/tutum/wordpress/) as a service image in your
`.gitlab-ci.yml`:

```yaml
services:
- tutum/wordpress:latest
```

When the build is run, `tutum/wordpress` will be started first and you will have
access to it from your build container under the hostname `tutum__wordpress`
and `tutum-wordpress`.

The GitLab Runner creates two alias hostnames for the service that you can use
alternatively. The aliases are taken from the image name following these rules:

1. Everything after `:` is stripped.
1. For the first alias, the slash (`/`) is replaced with double underscores (`__`).
1. For the second alias, the slash (`/`) is replaced with a single dash (`-`).

Using a private service image will strip any port given and apply the rules as
described above. A service `registry.gitlab-wp.com:4999/tutum/wordpress` will
result in hostname `registry.gitlab-wp.com__tutum__wordpress` and
`registry.gitlab-wp.com-tutum-wordpress`.

## Configuring services

Many services accept environment variables which allow you to easily change
database names or set account names depending on the environment.

GitLab Runner 0.5.0 and up passes all YAML-defined variables to the created
service containers.

For all possible configuration variables check the documentation of each image
provided in their corresponding Docker hub page.

NOTE: **Note**:
All variables will be passed to all services containers. It's not designed to
distinguish which variable should go where.
Secure variables are only passed to the build container.

## Mounting a directory in RAM

You can mount a path in RAM using tmpfs. This can speed up the time required to test if there is a lot of I/O related work, such as with databases.
If you use the `tmpfs` and `services_tmpfs` options in the runner configuration, you can specify multiple paths, each with its own options. See the [docker reference](https://docs.docker.com/engine/reference/commandline/run/#mount-tmpfs-tmpfs) for details.
This is an example `config.toml` to mount the data directory for the official Mysql container in RAM.

```toml
[runners.docker]
  # For the main container
  [runners.docker.tmpfs]
      "/var/lib/mysql" = "rw,noexec"

  # For services
  [runners.docker.services_tmpfs]
      "/var/lib/mysql" = "rw,noexec"
```

## Build directory in service

Since version 1.5 GitLab Runner mounts a `/builds` directory to all shared services.

See an issue: <https://gitlab.com/gitlab-org/gitlab-runner/issues/1520>.

### PostgreSQL service example

See the specific documentation for
[using PostgreSQL as a service](https://gitlab.com/gitlab-org/gitlab-ce/blob/master/doc/ci/services/postgres.md).

### MySQL service example

See the specific documentation for
[using MySQL as a service](https://gitlab.com/gitlab-org/gitlab-ce/blob/master/doc/ci/services/mysql.md).

### The services health check

After the service is started, GitLab Runner waits some time for the service to
be responsive. Currently, the Docker executor tries to open a TCP connection to
the first exposed service in the service container.

You can see how it is implemented by checking this [Go command](https://gitlab.com/gitlab-org/gitlab-runner/blob/master/commands/helpers/health_check.go).

## The builds and cache storage

The Docker executor by default stores all builds in
`/builds/<namespace>/<project-name>` and all caches in `/cache` (inside the
container).
You can overwrite the `/builds` and `/cache` directories by defining the
`builds_dir` and `cache_dir` options under the `[[runners]]` section in
`config.toml`. This will modify where the data are stored inside the container.

If you modify the `/cache` storage path, you also need to make sure to mark this
directory as persistent by defining it in `volumes = ["/my/cache/"]` under the
`[runners.docker]` section in `config.toml`.

### Clearing Docker cache

GitLab Runner provides the [`clear-docker-cache`](https://gitlab.com/gitlab-org/gitlab-runner/blob/master/packaging/root/usr/share/gitlab-runner/clear-docker-cache)
script to remove containers that can unnecessarily consume disk space.

Run `clear-docker-cache` regularly (using `cron` once per week, for example),
ensuring a balance is struck between:

- Maintaining some recent containers in the cache for performance.
- Reclaiming disk space.

NOTE: **Note:**
`clear-docker-cache` does not clean build or cache volumes.

## The persistent storage

The Docker executor can provide a persistent storage when running the containers.
All directories defined under `volumes =` will be persistent between builds.

The `volumes` directive supports two types of storage:

1. `<path>` - **the dynamic storage**. The `<path>` is persistent between subsequent
   runs of the same concurrent job for that project. The data is attached to a
   custom cache container: `runner-<short-token>-project-<id>-concurrent-<job-id>-cache-<unique-id>`.
1. `<host-path>:<path>[:<mode>]` - **the host-bound storage**. The `<path>` is
   bind to `<host-path>` on the host system. The optional `<mode>` can specify
   that this storage is read-only or read-write (default).

### The persistent storage for builds

If you make the `/builds` to be **the host-bound storage**, your builds will be stored in:
`/builds/<short-token>/<concurrent-id>/<namespace>/<project-name>`, where:

- `<short-token>` is a shortened version of the Runner's token (first 8 letters)
- `<concurrent-id>` is a unique number, identifying the local job ID on the
  particular Runner in context of the project

## The privileged mode

The Docker executor supports a number of options that allows to fine tune the
build container. One of these options is the [`privileged` mode][privileged].

### Use docker-in-docker with privileged mode

The configured `privileged` flag is passed to the build container and all
services, thus allowing to easily use the docker-in-docker approach.

First, configure your Runner (config.toml) to run in `privileged` mode:

```toml
[[runners]]
  executor = "docker"
  [runners.docker]
    privileged = true
```

Then, make your build script (`.gitlab-ci.yml`) to use Docker-in-Docker
container:

```yaml
image: docker:git
services:
- docker:dind

build:
  script:
  - docker build -t my-image .
  - docker push my-image
```

## The ENTRYPOINT

The Docker executor doesn't overwrite the [`ENTRYPOINT` of a Docker image][entry].

That means that if your image defines the `ENTRYPOINT` and doesn't allow to run
scripts with `CMD`, the image will not work with the Docker executor.

With the use of `ENTRYPOINT` it is possible to create special Docker image that
would run the build script in a custom environment, or in secure mode.

You may think of creating a Docker image that uses an `ENTRYPOINT` that doesn't
execute the build script, but does execute a predefined set of commands, for
example to build the docker image from your directory. In that case, you can
run the build container in [privileged mode](#the-privileged-mode), and make
the build environment of the Runner secure.

Consider the following example:

1. Create a new Dockerfile:

   ```dockerfile
   FROM docker:dind
   ADD / /entrypoint.sh
   ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
   ```

1. Create a bash script (`entrypoint.sh`) that will be used as the `ENTRYPOINT`:

   ```shell
   #!/bin/sh

   dind docker daemon
       --host=unix:///var/run/docker.sock \
       --host=tcp://0.0.0.0:2375 \
       --storage-driver=vf &

   docker build -t "$BUILD_IMAGE" .
   docker push "$BUILD_IMAGE"
   ```

1. Push the image to the Docker registry.

1. Run Docker executor in `privileged` mode. In `config.toml` define:

   ```toml
   [[runners]]
     executor = "docker"
     [runners.docker]
       privileged = true
   ```

1. In your project use the following `.gitlab-ci.yml`:

   ```yaml
   variables:
     BUILD_IMAGE: my.image
   build:
     image: my/docker-build:image
     script:
     - Dummy Script
   ```

This is just one of the examples. With this approach the possibilities are
limitless.

## How pull policies work

When using the `docker` or `docker+machine` executors, you can set the
`pull_policy` parameter which defines how the Runner will work when pulling
Docker images (for both `image` and `services` keywords).

NOTE: **Note:**
If you don't set any value for the `pull_policy` parameter, then
Runner will use the `always` pull policy as the default value.

Now let's see how these policies work.

### Using the `never` pull policy

The `never` pull policy disables images pulling completely. If you set the
`pull_policy` parameter of a Runner to `never`, then users will be able
to use only the images that have been manually pulled on the docker host
the Runner runs on.

If an image cannot be found locally, then the Runner will fail the build
with an error similar to:

```plaintext
Pulling docker image local_image:latest ...
ERROR: Build failed: Error: image local_image:latest not found
```

**When to use this pull policy?**

This pull policy should be used if you want or need to have a full
control on which images are used by the Runner's users. It is a good choice
for private Runners that are dedicated to a project where only specific images
can be used (not publicly available on any registries).

**When not to use this pull policy?**

This pull policy will not work properly with most of [auto-scaled](../configuration/autoscale.md)
Docker executor use cases. Because of how auto-scaling works, the `never`
pull policy may be usable only when using a pre-defined cloud instance
images for chosen cloud provider. The image needs to contain installed
Docker Engine and local copy of used images.

### Using the `if-not-present` pull policy

When the `if-not-present` pull policy is used, the Runner will first check
if the image is present locally. If it is, then the local version of
image will be used. Otherwise, the Runner will try to pull the image.

**When to use this pull policy?**

This pull policy is a good choice if you want to use images pulled from
remote registries but you want to reduce time spent on analyzing image
layers difference, when using heavy and rarely updated images.
In that case, you will need once in a while to manually remove the image
from the local Docker Engine store to force the update of the image.

It is also the good choice if you need to use images that are built
and available only locally, but on the other hand, also need to allow to
pull images from remote registries.

**When not to use this pull policy?**

This pull policy should not be used if your builds use images that
are updated frequently and need to be used in most recent versions.
In such situation, the network load reduction created by this policy may
be less worthy than the necessity of the very frequent deletion of local
copies of images.

This pull policy should also not be used if your Runner can be used by
different users which should not have access to private images used
by each other. Especially do not use this pull policy for shared Runners.

To understand why the `if-not-present` pull policy creates security issues
when used with private images, read the
[security considerations documentation][secpull].

### Using the `always` pull policy

The `always` pull policy will ensure that the image is **always** pulled.
When `always` is used, the Runner will try to pull the image even if a local
copy is available. If the image is not found, then the build will
fail with an error similar to:

```plaintext
Pulling docker image registry.tld/my/image:latest ...
ERROR: Build failed: Error: image registry.tld/my/image:latest not found
```

When using the `always` pull policy in GitLab Runner versions older than `v1.8`, it could
fall back to the local copy of an image and print a warning:

```plaintext
Pulling docker image registry.tld/my/image:latest ...
WARNING: Cannot pull the latest version of image registry.tld/my/image:latest : Error: image registry.tld/my/image:latest not found
WARNING: Locally found image will be used instead.
```

This was [changed in GitLab Runner `v1.8`](https://gitlab.com/gitlab-org/gitlab-runner/issues/1905).

**When to use this pull policy?**

This pull policy should be used if your Runner is publicly available
and configured as a shared Runner in your GitLab instance. It is the
only pull policy that can be considered as secure when the Runner will
be used with private images.

This is also a good choice if you want to force users to always use
the newest images.

Also, this will be the best solution for an [auto-scaled](../configuration/autoscale.md)
configuration of the Runner.

**When not to use this pull policy?**

This pull policy will definitely not work if you need to use locally
stored images. In this case, the Runner will skip the local copy of the image
and try to pull it from the remote registry. If the image was build locally
and doesn't exist in any public registry (and especially in the default
Docker registry), the build will fail with:

```plaintext
Pulling docker image local_image:latest ...
ERROR: Build failed: Error: image local_image:latest not found
```

## Docker vs Docker-SSH (and Docker+Machine vs Docker-SSH+Machine)

NOTE: **Note**:
Starting with GitLab Runner 10.0, both Docker-SSH and Docker-SSH+machine executors
are **deprecated** and will be removed in one of the upcoming releases.

We provided a support for a special type of Docker executor, namely Docker-SSH
(and the autoscaled version: Docker-SSH+Machine). Docker-SSH uses the same logic
as the Docker executor, but instead of executing the script directly, it uses an
SSH client to connect to the build container.

Docker-SSH then connects to the SSH server that is running inside the container
using its internal IP.

This executor is no longer maintained and will be removed in the near future.

[Docker Fundamentals]: https://docs.docker.com/engine/understanding-docker/
[docker engine]: https://www.docker.com/products/container-runtime
[hub]: https://hub.docker.com/
[linking-containers]: https://docs.docker.com/engine/userguide/networking/default_network/dockerlinks/
[postgres-hub]: https://registry.hub.docker.com/u/library/postgres/
[mysql-hub]: https://registry.hub.docker.com/u/library/mysql/
[runner-priv-reg]: ../configuration/advanced-configuration.md#using-a-private-container-registry
[yaml]: http://doc.gitlab.com/ce/ci/yaml/README.html
[toml]: ../commands/README.md#configuration-file
[alpine linux]: https://alpinelinux.org/
[special-build]: https://gitlab.com/gitlab-org/gitlab-runner/tree/master/dockerfiles/build
[privileged]: https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities
[entry]: https://docs.docker.com/engine/reference/run/#entrypoint-default-command-to-execute-at-runtime
[secpull]: ../security/index.md#usage-of-private-docker-images-with-if-not-present-pull-policy
