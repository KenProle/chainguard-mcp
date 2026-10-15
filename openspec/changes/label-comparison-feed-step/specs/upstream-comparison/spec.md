# Spec Delta

## MODIFIED Requirements

### Requirement: UC-9 Progress while loading
While a comparison is being computed, the web UI SHALL show its progress with the number of packages checked out of the total, and SHALL never show only a loading message for it. Once every package is checked, while the current status of the Chainguard image's advisories is read from Chainguard's advisory feed (UC-17), it SHALL say so.

#### Scenario: UC-9.1 First comparison
- **WHEN** `node:20` is compared and nothing is cached
- **THEN** a progress bar shows packages checked out of the total until the comparison appears

#### Scenario: UC-9.2 Reading the advisory feed
- **WHEN** all 221 packages of a `node:20` comparison are checked and the feed is still being read
- **THEN** the progress bar is full and reads "Checking the current status of Chainguard's advisories…" instead of "221 of 221 packages checked"
