# Spec Delta

## Purpose

Makes sure every distributed copy of the server (the binary, a release archive, the Docker image) carries the license notices of the third-party code and data it contains, and that those notices stay in step with the dependencies as they change.

LN-1 to LN-3 cover what the notices contain and how they are read; LN-4 and LN-5 cover where they ship; LN-6 and LN-7 cover keeping them complete. The examples come from a build of `main` at `787db66` on 2026-11-10 (Go modules from `go version -m`, UI packages from a Vite build).

## ADDED Requirements

### Requirement: LN-1 Notices for everything the binary contains
The binary SHALL carry the license notices of all third-party code and data built into it: the Go standard library and runtime, every Go module linked into it, the dfc package mappings, and every package bundled into its web UI. Each entry SHALL give the component's name, its version where it has one, and the full text of its license files, including any `PATENTS` or `NOTICE` file.

#### Scenario: LN-1.1 A linked Go module with a PATENTS file
- **WHEN** the notices are printed for a binary built with `golang.org/x/sync` v0.20.0
- **THEN** they include an entry for `golang.org/x/sync` v0.20.0 with the text of both its `LICENSE` (BSD-3, "Copyright 2009 The Go Authors") and its `PATENTS` file

#### Scenario: LN-1.2 A module with a dual-license transition notice
- **WHEN** the notices are printed for a binary built with `github.com/modelcontextprotocol/go-sdk` v1.8.0
- **THEN** its entry includes the module's whole `LICENSE` file, from the paragraph on the transition from MIT to Apache-2.0 through both license texts

#### Scenario: LN-1.3 The Go standard library
- **WHEN** the notices are printed
- **THEN** they include an entry for the Go standard library and runtime with the text of the Go distribution's `LICENSE` and `PATENTS` files

#### Scenario: LN-1.4 The dfc mappings
- **WHEN** the notices are printed
- **THEN** they include an entry for Chainguard's dfc package mappings, naming the dfc commit the mappings come from, with the Apache-2.0 text from `mappings/LICENSE-dfc`

#### Scenario: LN-1.5 A bundled UI package
- **WHEN** the notices are printed for a binary whose UI bundles `react-router` 8.4.0
- **THEN** they include an entry for `react-router` 8.4.0, identified as MIT, with the text of its license file

### Requirement: LN-2 Only what is actually shipped
The notices SHALL list a Go module only if it is linked into the binary, and a UI package only if code from it is in the UI bundle. Development tooling and packages that are installed but tree-shaken out of the bundle SHALL NOT be listed.

#### Scenario: LN-2.1 A tree-shaken runtime package
- **WHEN** `react-router` 8.4.0 depends on `@remix-run/route-pattern` 0.22.1 and `cookie-es` 3.1.1, but no code from either ends up in the UI bundle
- **THEN** neither package appears in the notices

#### Scenario: LN-2.2 Development tooling
- **WHEN** the notices are printed
- **THEN** no entry names `vite`, `vitest`, `typescript`, `jsdom` or `oxlint`

#### Scenario: LN-2.3 The Go module list matches the binary
- **WHEN** the module list from `go version -m` for the built binary is compared with the Go module entries in its notices
- **THEN** the two lists name the same modules at the same versions

### Requirement: LN-3 The -licenses flag
Running the server with `-licenses` SHALL print all of the notices to standard output and exit with status 0, without starting a server, reading the network or writing files. The output SHALL be plain text that starts with a line naming the project and its version.

#### Scenario: LN-3.1 Printing the notices
- **WHEN** a release binary for version 1.4.0 is run as `chainguard-mcp -licenses`
- **THEN** it prints the notices, starting with a line that names chainguard-mcp 1.4.0, and exits with status 0 without listening on any port

#### Scenario: LN-3.2 From the Docker image
- **WHEN** the image is run as `docker run --rm <image> -licenses`
- **THEN** the container prints the same notices and exits with status 0

### Requirement: LN-4 Notices file in release archives
Every release archive SHALL contain a file named `THIRD_PARTY_NOTICES` next to `README.md` and `LICENSE`, whose content is exactly what the archive's binary prints with `-licenses`.

#### Scenario: LN-4.1 A release archive
- **WHEN** the `linux_amd64` archive of a release is unpacked
- **THEN** it contains `THIRD_PARTY_NOTICES`, and its content equals the output of the unpacked binary's `-licenses`

### Requirement: LN-5 Notices file in the Docker image
The Docker image SHALL contain `/THIRD_PARTY_NOTICES`, whose content is exactly what the image's binary prints with `-licenses`.

#### Scenario: LN-5.1 The image's notices file
- **WHEN** `/THIRD_PARTY_NOTICES` is copied out of a container created from the image
- **THEN** its content equals the output of `docker run --rm <image> -licenses`

### Requirement: LN-6 Go notices kept in step with the module graph
The project's tests SHALL fail when the Go notices built into the binary don't match the modules the server links, at the versions `go.mod` selects, naming each module that is missing, extra or at a different version. Regenerating the notices SHALL take one documented command.

#### Scenario: LN-6.1 A dependency upgrade without regenerating
- **WHEN** `go.mod` moves `golang.org/x/time` from v0.15.0 to a newer version and the notices are not regenerated
- **THEN** `go test ./...` fails and the failure names `golang.org/x/time` and the command that regenerates the notices

#### Scenario: LN-6.2 A Go toolchain update
- **WHEN** only the `toolchain` line in `go.mod` changes
- **THEN** the notices tests still pass without regenerating, because the standard library's entry does not carry the toolchain version

### Requirement: LN-7 Missing UI notices fail the release build
A binary built with the web UI SHALL refuse to print incomplete notices: when the UI is built but its license data is missing, `-licenses` SHALL print an error to standard error and exit with a non-zero status, so the release and Docker builds fail. A binary built without the UI SHALL print the other notices and say that no UI is included.

#### Scenario: LN-7.1 UI built without license data
- **WHEN** a binary whose embedded UI has `index.html` but no license data is run with `-licenses`
- **THEN** it prints an error saying the UI's license notices are missing and exits with a non-zero status

#### Scenario: LN-7.2 Binary built without the UI
- **WHEN** a binary built with only `web/dist/.gitkeep` (no Node build) is run with `-licenses`
- **THEN** it prints the Go and dfc notices and a line saying the web UI is not included in this build, and exits with status 0
