# Measurement (tasks 1.1, 7.3 and 7.4)

Run on 2026-10-19 against this branch's build (`-http 127.0.0.1:8098`; 8080 was taken by another process), Wolfi's live index and dfc commit `84e5c1c`.

## 1.1 Repository of the free `-dev` images

- `/etc/apk/repositories` in `cgr.dev/chainguard/python:latest-dev` and `node:latest-dev` is `https://apk.cgr.dev/chainguard`. Read anonymously, that repository lists the same 15,799 package names as `packages.wolfi.dev`.
- `apk add --no-cache build-base postgresql-18-dev` in `python:latest-dev` succeeds.

## 7.3 The prompt and the tool together

**Setup.** I got the `migrate_dockerfile` prompt over the build's own MCP endpoint (`/mcp`), with a Dockerfile that runs `apt-get install -y build-essential libpq-dev`.

**The prompt.** Step 4 names `map_packages`.

**The assistant run.** Claude Code (`claude -p`, Sonnet 5.5) was connected to the same endpoint, so the run went over Streamable HTTP rather than stdio. The assistant:
- called `map_packages`
- used `build-base` and `postgresql-18-dev`
- listed `postgresql-18-dev` under things to check: a candidate found through the pkg-config module `libpq`, and it noted that bookworm's `libpq-dev` is PostgreSQL 15

## 7.4 With and without `map_packages`

### Method

**Dockerfiles.** There were three. They follow common real-world patterns and were written for this measurement, not fetched:
- a cookiecutter-Django-style Python build
- a Node app with native modules
- the Rails 7 generated Dockerfile

**Runs.** Each Dockerfile was migrated twice by `claude -p` on Sonnet 5.5, with only this build's MCP tools and no built-in tools:
- **without:** the prompt from `main` (step 4: "look up package names where they differ"), with `map_packages` disallowed
- **with:** this branch's prompt, with `map_packages` available

**Check.** Each run's `apk add` line was then run in the matching `-dev` image.

### Results

| Dockerfile | Arm | `apk add` | Result |
|---|---|---|---|
| Python | without | `build-base postgresql-dev libpq gettext` | installs (`postgresql-dev` and `libpq` are virtual names; apk picks `postgresql-18-dev` and `libpq-18`) |
| Python | with | `build-base postgresql-18-dev gettext wait-for-it` | installs |
| Node | without | `python-3` (it noted that `make`, `gcc`, `git` and `dumb-init` are already in `node:latest-dev`, which is true) | installs (`python-3` is virtual; apk picks `python-3.13`) |
| Node | with | `python-3.14 make gcc git ca-certificates dumb-init` | installs (four are redundant) |
| Rails | without | `curl jemalloc vips libpq postgresql-client` and `build-base git libpq-dev yaml-dev pkgconf` | **fails**: `libpq-dev` and `vips` don't exist |
| Rails | with | `build-base git postgresql-18-dev yaml-dev pkgconf`, and `libvips libjemalloc2 postgresql-18-client curl` | installs |

**Wrong package names: 2 without (one Dockerfile out of three fails to build), 0 with.**

The cost was $0.05–0.08 and about 30 seconds per run, with no clear difference between the arms.

### What this shows

- **The gain is real but smaller than the coverage numbers suggested.** Without the tool, the model already knows most mappings. It also uses Wolfi's virtual names (`postgresql-dev`, `python-3`, `postgresql-client`), which `apk add` resolves. The tool's value is catching the names that are simply wrong (`libpq-dev`, `vips`) and making every version choice visible.
- **Virtual names are installable, and the tool doesn't treat them that way.**
  - The index check (PM-2) and the drift report count only package names. So dfc's `postgresql-dev` and `python-3`, which the design called stale, actually install.
  - The model without the tool sometimes did better here: `python-3` lets Wolfi pick its default stream (3.13), where the tool chose 3.14.
  - That's a follow-up for the spec, not a defect in what was specified.
- **The `-dev` images already have build tools.** The tool maps every name it's given. Saying which packages a `-dev` image already contains is outside its scope.

### Limits

- One run per arm. Model output varies between runs.
- Three Dockerfiles, written from common patterns.
- Only the package names were scored, not the rest of each migration.
