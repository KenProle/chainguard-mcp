# Tasks

Commands are for Git Bash on Windows as in CLAUDE.md. Test names start with the scenario ID they cover. No design.md: the change is small and has no open technical decisions.

## 1. Server

- [x] 1.1 Refuse names over 128 characters in `validateMapNames`, naming the start of the value (PM-9). Verify with `TestPM9_5_NameTooLong`: 129 characters is refused as invalid input, and 128 characters is mapped.
- [x] 1.2 In `ruleStreams`, collect the packages starting with the name's first segment once, and match each prefix against those only. Verify: the PM-7 tests still pass.
- [x] 1.3 Stop `MapPackages` between names when the context is cancelled. Verify with `TestPM9_5_CancelledRequest`: a cancelled context returns its error.
- [x] 1.4 Serve HTTP mode with an `http.Server` setting `ReadHeaderTimeout` 10 s, `IdleTimeout` 2 min and `MaxHeaderBytes` 64 KB.

## 2. Verification

- [x] 2.1 Run `gofmt` on the changed files (`tr -d '\r' < file | gofmt -l`), `go vet ./...`, `go test ./...` and `openspec validate --all --strict`. Verify: all pass.
- [x] 2.2 In a running server, time `GET /api/packages/map` with a 128-character hyphenated name made of a repeated made-up segment, and a 129-character one. Verify: the first answers in well under a second, the second is 400 `invalid_input`.
