# dummy-container-app

Minimal Go web server for testing build/deploy plumbing.

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
