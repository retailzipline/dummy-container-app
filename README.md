# dummy-container-app

Minimal Go app for testing container build/deploy plumbing. One binary, two
commands.

## Commands

The image's entrypoint is the binary; the subcommand selects the process.

| Command | What it does |
| --- | --- |
| `server` (default) | Serves `GET /info` on `$PORT` (default `8080`). |
| `worker` | Logs `Ah, ha, ha, ha, stayin' alive, stayin' alive` every 5s. |

```sh
docker run --rm -p 8080:8080 dummy-container-app:latest          # server
docker run --rm dummy-container-app:latest worker                # worker
docker run --rm -e WORKER_INTERVAL=1s dummy-container-app:latest worker
```

`WORKER_INTERVAL` takes any Go duration (`500ms`, `5s`, `1m`). Both commands
handle `SIGTERM`, so the container stops promptly instead of waiting out the
runtime's grace period and being killed.

An unrecognized subcommand exits 1 with a message naming the valid ones.

## Endpoint

`GET /info` →

```json
{
  "version": "abc1234",
  "build_secret": "s3cr3t-from-build",
  "hostname": "e92816e1b732"
}
```

- `version` — contents of `.version` in the working directory, or `"unknown"` if the file is missing/empty.
- `build_secret` — contents of `.build-secret`, written at image build time from the `TEST_BUILD_SECRET` build secret. `"unknown"` when no secret was supplied.
- `hostname` — container hostname, handy for spotting which replica answered.

Listens on `$PORT` (default `8080`).

## Base images

Both stages use [Docker Hardened Images](https://docs.docker.com/dhi/) from the
`dhi.io` registry:

| Stage | Image | Why |
| --- | --- | --- |
| builder | `dhi.io/golang:1.25-dev` | `-dev` variant runs as root and ships the Go toolchain and a shell, which the `RUN` steps need. |
| runner | `dhi.io/static:20260909-alpine` | Minimal, no shell, no package manager, runs as nonroot `65532`. Works because the binary is fully static (`CGO_ENABLED=0`). |

The `static` base is 0.25 MB against 4.13 MB for `alpine:3.22`; the final image
is ~10 MB. `static` tags are date-stamped rather than semver — there is no
`latest` — so bump the date deliberately when you want a newer base.

Because the runner has no shell, `.version` and `.build-secret` are written in
the **builder** stage and copied across; `RUN` is not available in the final
stage.

`dhi.io` is an authenticated registry. Log in with Docker Hub credentials before
building:

```sh
docker login dhi.io
```

## Build

```sh
TEST_BUILD_SECRET='s3cr3t-from-build' docker buildx build \
  --secret id=TEST_BUILD_SECRET,env=TEST_BUILD_SECRET \
  --build-arg GIT_SHA="$(git rev-parse --short HEAD)" \
  -t dummy-container-app:latest --load .
```

Both the build arg and the secret are optional — omit either and the
corresponding field reports `"unknown"`.

## Run

```sh
docker run --rm -p 8080:8080 dummy-container-app:latest
curl -s localhost:8080/info
```

> The build secret is written to a plain file inside the image so `/info` can
> echo it back. That is deliberate for this test app and must never be done with
> a real secret — it persists in the image layer.

## CI

`.github/workflows/build.yml` validates the image on every push to `main` and
every pull request: it builds with a `GIT_SHA` build arg and a
`TEST_BUILD_SECRET` build secret, boots the container, and asserts `/info`
reports the expected version and a non-`unknown` secret. It uses the repository
secret `TEST_BUILD_SECRET` when one is configured, otherwise a dummy value, so
the workflow is green without any repo setup.

It also runs `docker/login-action` against `dhi.io` before building, since the
hardened base images are not anonymously pullable. That step needs two repo
secrets: `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` (a Docker Hub PAT with read
access). **Without them the build fails at the first `FROM`** — this is the one
piece of required repo setup.

### Build-cache caveat

BuildKit deliberately keeps secret *values* out of the layer cache key. Rebuild
the same `GIT_SHA` with a different `TEST_BUILD_SECRET` and the cached layer is
reused, so the image keeps the **old** secret:

```sh
# both of these produce an image reporting the first secret
docker buildx build --build-arg GIT_SHA=abc1234 --secret id=TEST_BUILD_SECRET,env=TEST_BUILD_SECRET ...
TEST_BUILD_SECRET=rotated docker buildx build --build-arg GIT_SHA=abc1234 ...
```

In practice `GIT_SHA` changes per commit and invalidates the chain, but CI runs
with `no-cache: true` so a stale layer can never satisfy the smoke test. Locally,
pass `--no-cache` (or change `GIT_SHA`) when rotating the secret.
