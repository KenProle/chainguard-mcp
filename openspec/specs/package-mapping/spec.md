# package-mapping Specification

## Purpose

Maps the Debian and Ubuntu (apt) package names in a Dockerfile to the Wolfi apk packages that replace them, checked against Wolfi's live package index and saying where each answer came from, so a migration to Chainguard images doesn't rest on guessed names.

PM-1 to PM-3 define the request and result; PM-4 to PM-9 how each name is resolved; PM-10 and PM-11 the package index; PM-12 to PM-14 the MCP tool, the API endpoint and the migrate prompt; PM-15 to PM-17 the embedded dfc mappings and their refresh. Examples were measured against Wolfi's x86_64 index on 2026-10-19 and dfc's mapping file of the same day.

## Requirements

### Requirement: PM-1 Map a list of apt package names
Given a list of apt package names, the mapping SHALL return one result per distinct name, in the order given. Each result SHALL give the name asked for, a status (`exact`, `mapped`, `candidates` or `unknown`), the Wolfi packages to install, any alternatives, and the source of the answer. The response SHALL also give when the package index was fetched and which dfc mapping snapshot was used.

#### Scenario: PM-1.1 Several names
- **WHEN** the names `curl`, `build-essential` and `libpq-dev` are mapped
- **THEN** there are three results, for `curl`, `build-essential` and `libpq-dev` in that order
- **AND** the response gives the index fetch time and the dfc snapshot's upstream commit

#### Scenario: PM-1.2 Duplicate names
- **WHEN** the names `curl`, `curl` are mapped
- **THEN** there is one result, for `curl`

### Requirement: PM-2 Only indexed packages are returned
Every package named in a result's packages or alternatives SHALL exist in the package index in use at the time of the request. A name SHALL NOT be returned only because a hint or rule produced it.

#### Scenario: PM-2.1 A stale dfc hint
- **WHEN** `libpq-dev` is mapped, which dfc maps to `postgresql-dev`, a name not in the index on 2026-10-19
- **THEN** the result doesn't name `postgresql-dev`

#### Scenario: PM-2.2 Every result checked
- **WHEN** any list of names is mapped
- **THEN** every package in every result's packages and alternatives is in the index

### Requirement: PM-3 Statuses and sources
`exact` SHALL mean the apt name is itself a Wolfi package; `mapped` SHALL mean a dfc mapping gave the packages; `candidates` SHALL mean a rule found them and they need checking; `unknown` SHALL mean nothing was found, with no packages. Each result other than `unknown` SHALL name its source, such as the index, dfc, or the rule or provided item that matched.

#### Scenario: PM-3.1 Exact
- **WHEN** `curl` is mapped
- **THEN** the status is `exact`, the packages are `curl` and the source is the index

#### Scenario: PM-3.2 Unknown
- **WHEN** `dnsutils` is mapped
- **THEN** the status is `unknown` and no packages are given

### Requirement: PM-4 Resolution order
Each name SHALL be resolved by the first of these steps that gives a package in the index: the name itself (`exact`); its dfc mapping (`mapped`); the rules (`candidates`). A name no step resolves SHALL be `unknown`. A dfc mapping to an empty list SHALL be `mapped` with no packages and a note that dfc lists no replacement.

#### Scenario: PM-4.1 The name exists
- **WHEN** `libffi-dev` is mapped, which is also a Wolfi package
- **THEN** the status is `exact`, without consulting dfc or the rules

#### Scenario: PM-4.2 A dfc mapping
- **WHEN** `build-essential` is mapped
- **THEN** the status is `mapped`, the packages are `build-base` and the source is dfc

#### Scenario: PM-4.3 Several packages for one name
- **WHEN** `ssh` is mapped
- **THEN** the status is `mapped` and the packages are `openssh-client` and `openssh-server`

#### Scenario: PM-4.4 Falling through a stale mapping
- **WHEN** `libpq-dev` is mapped, whose only dfc target is not in the index
- **THEN** the result comes from the rules, with status `candidates`

#### Scenario: PM-4.5 No replacement needed
- **WHEN** `software-properties-common` is mapped, which dfc maps to an empty list
- **THEN** the status is `mapped`, with no packages and a note that dfc lists no replacement

### Requirement: PM-5 Development packages keep their headers
When a name ending in `-dev` would map to a package that doesn't end in `-dev`, and the index has that package's `-dev` package or its origin's `-dev` package, the mapping SHALL return the `-dev` package instead and say in the source that it corrected the mapping.

#### Scenario: PM-5.1 libssl-dev
- **WHEN** `libssl-dev` is mapped, which dfc maps to the runtime package `libssl3` (origin `openssl`)
- **THEN** the status is `mapped`, the packages are `openssl-dev`, and the source says the dfc mapping was corrected

#### Scenario: PM-5.2 libsqlite3-dev
- **WHEN** `libsqlite3-dev` is mapped, which dfc maps to `sqlite-libs`
- **THEN** the packages are `sqlite-dev`

### Requirement: PM-6 Rules
When neither the name nor a dfc mapping gives a package, the rules SHALL look for packages that provide what the apt package is named for: a virtual package of the same name or of its last word, a pkg-config module for a `lib<x>-dev` name, a shared library for a `lib<x><N>` name, or a command. They SHALL also try the `python3-<x>` to `py3-<x>` naming and the name without a leading `default-`.

#### Scenario: PM-6.1 pkg-config module
- **WHEN** `libpq-dev` is mapped
- **THEN** the status is `candidates` and the source names the pkg-config module `libpq`

#### Scenario: PM-6.2 Shared library
- **WHEN** `libgomp1` is mapped
- **THEN** the status is `candidates`, the packages are `libgomp` and the source names the library `libgomp.so.1`

#### Scenario: PM-6.3 Command
- **WHEN** `iputils-ping` is mapped
- **THEN** the status is `candidates`, the packages are `iputils` and the source names the command `ping`

#### Scenario: PM-6.4 Without default-
- **WHEN** `default-libmysqlclient-dev` is mapped
- **THEN** the status is `candidates` and the packages are a `mysql-<version>-dev` package

#### Scenario: PM-6.5 Virtual package of the last word
- **WHEN** `golang-go` is mapped, and `gcc-go` is the only package with a `go` command while the `go-1.<N>` streams provide the virtual package `go`
- **THEN** the status is `candidates`, the packages are one `go-1.<N>` stream and the source names the virtual package `go`, not `gcc-go`

### Requirement: PM-7 Version streams
When the matches are version streams of one package (`nodejs-24`, `nodejs-26`), the mapping SHALL choose the highest version among streams that are actively built and not prereleases, list the other streams as alternatives, and add a note that a version was chosen and should match the version the Dockerfile used.

#### Scenario: PM-7.1 PostgreSQL client headers
- **WHEN** `libpq-dev` is mapped
- **THEN** the packages are `postgresql-18-dev`, and the alternatives include `postgresql-17-dev` and `postgresql-16-dev`
- **AND** the note says a version was chosen

#### Scenario: PM-7.2 A prerelease stream isn't chosen
- **WHEN** `python3` is mapped, and the index's newest build is `python-3.15` at version `3.15.0_git20261017`
- **THEN** the packages are not `python-3.15`, which is listed as an alternative

#### Scenario: PM-7.3 A stream no longer built isn't chosen
- **WHEN** `postgresql-client` is mapped, and `postgresql-12-client` was last built in March 2024
- **THEN** the packages are `postgresql-18-client`

### Requirement: PM-8 Unknown names
A name no step resolves SHALL be `unknown`. It SHALL NOT be given a best guess.

#### Scenario: PM-8.1 dnsutils
- **WHEN** `dnsutils` is mapped
- **THEN** the status is `unknown` with no packages and no alternatives

### Requirement: PM-9 Input validation
A request SHALL be refused as invalid input when it has no names, more than 100 names, or a name that isn't a valid Debian package name (lowercase letters, digits and `+`, `-` or `.`, starting with a letter or digit, at least 2 characters). The error SHALL name the first offending value.

#### Scenario: PM-9.1 Invalid name
- **WHEN** the names `curl` and `Curl;rm` are mapped
- **THEN** the request is refused as invalid input naming `Curl;rm`

#### Scenario: PM-9.2 Too many names
- **WHEN** 101 names are mapped
- **THEN** the request is refused as invalid input

#### Scenario: PM-9.3 No names
- **WHEN** an empty list is mapped
- **THEN** the request is refused as invalid input

### Requirement: PM-10 Package index source and caching
Packages SHALL be looked up in Wolfi's public x86_64 package index, read anonymously. The index SHALL be kept for 6 hours. When a refresh fails, the last copy SHALL be used, and the response's fetch time SHALL show its age.

#### Scenario: PM-10.1 Cached index
- **WHEN** two requests are made 10 minutes apart
- **THEN** the index is downloaded once

#### Scenario: PM-10.2 Refresh fails
- **WHEN** the index was fetched 7 hours ago and the refresh fails
- **THEN** names are mapped against the older copy, and the fetch time is 7 hours ago

### Requirement: PM-11 Package index unavailable
When the package index can't be read and no copy is held, the mapping SHALL fail as a source failure naming the Wolfi package index, and SHALL NOT return mappings from dfc or the rules alone.

#### Scenario: PM-11.1 No index
- **WHEN** the index can't be downloaded and none was fetched before
- **THEN** the request fails as a source failure naming the Wolfi package index, with no results

### Requirement: PM-12 MCP tool
An MCP tool `map_packages` SHALL take a list of apt package names and return the mapping result. Its description SHALL say that names are checked against Wolfi's package index, that `candidates` need checking, and that a mapped name doesn't by itself mean the Dockerfile will build.

#### Scenario: PM-12.1 Tool call
- **WHEN** a client calls `map_packages` with `["build-essential"]`
- **THEN** the result has one entry, `build-essential`, mapped to `build-base`

#### Scenario: PM-12.2 Tool error
- **WHEN** a client calls `map_packages` with `["Curl;rm"]`
- **THEN** the tool returns an error naming `Curl;rm`

### Requirement: PM-13 API endpoint
`GET /api/packages/map` SHALL take the names as a comma-separated `names` query parameter and return the same result as the MCP tool. Invalid input SHALL be 400 `invalid_input`, and an unavailable package index SHALL be 502 `source_failed` with the source named. The endpoint SHALL be listed in `GET /api/` and in the README's endpoint table.

#### Scenario: PM-13.1 Request
- **WHEN** a client sends `GET /api/packages/map?names=curl,build-essential`
- **THEN** the response is 200 with two results: `curl` `exact` and `build-essential` `mapped`

#### Scenario: PM-13.2 Invalid name
- **WHEN** a client sends `GET /api/packages/map?names=Curl;rm`
- **THEN** the response is 400 with code `invalid_input`

#### Scenario: PM-13.3 Index unavailable
- **WHEN** the package index can't be read and no copy is held
- **THEN** the response is 502 with code `source_failed`, naming the Wolfi package index

#### Scenario: PM-13.4 Listed in the index
- **WHEN** a client sends `GET /api/`
- **THEN** the index has an entry for `/api/packages/map` with `names` as a required query parameter

### Requirement: PM-14 Migrate prompt uses the mapping
The `migrate_dockerfile` prompt SHALL tell the assistant to map the apt packages in the Dockerfile with `map_packages` instead of guessing names, to use the packages it returns, and to list every `candidates` and `unknown` result among the things that need manual checking.

#### Scenario: PM-14.1 Prompt text
- **WHEN** a client gets the `migrate_dockerfile` prompt
- **THEN** its steps name `map_packages` for apt package names
- **AND** say that `candidates` and `unknown` results go in the list of things to check

### Requirement: PM-15 Embedded dfc mappings
The server SHALL embed dfc's package mapping file unchanged, together with the upstream commit it came from and dfc's license. Only its Debian package mappings SHALL be used. The README SHALL credit dfc and its license, and SHALL NOT imply the project is affiliated with Chainguard.

#### Scenario: PM-15.1 Provenance
- **WHEN** names are mapped
- **THEN** the response's dfc snapshot gives the upstream commit of the embedded file

#### Scenario: PM-15.2 File shape checked
- **WHEN** the test suite runs
- **THEN** it fails if the embedded file can't be read as dfc's mapping format or has no Debian mappings

### Requirement: PM-16 Refreshing the dfc mappings
A `-refresh-mappings <file>` run mode SHALL download dfc's mapping file at its latest upstream commit, write it and that commit to the given file, print the entries added and removed, and exit without starting the server. A downloaded file that isn't in dfc's mapping format SHALL be refused, leaving the existing file unchanged.

#### Scenario: PM-16.1 Refresh
- **WHEN** `-refresh-mappings` runs and dfc has a newer commit
- **THEN** the file holds the newer mapping file and its commit, and the added and removed entries are printed

#### Scenario: PM-16.2 Unexpected format
- **WHEN** the downloaded file isn't in dfc's mapping format
- **THEN** the run fails and the existing file is unchanged

### Requirement: PM-17 Drift report
The live tests SHALL report, without failing, how far the embedded dfc snapshot is behind upstream, which of its Debian targets are missing from the live index, and which of its mappings the `-dev` rule corrects.

#### Scenario: PM-17.1 Live run
- **WHEN** the live tests run with `CHAINGUARD_LIVE=1`
- **THEN** they print the snapshot's age against upstream, the missing targets (such as `postgresql-dev`) and the corrected mappings (such as `libssl-dev`)
