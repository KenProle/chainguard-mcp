# Spec Delta

## MODIFIED Requirements

### Requirement: SF-7 No claim of being vulnerability-free
The fixes chart and its surrounding text SHALL describe what the Wolfi security database records, and SHALL NOT state or imply that an image or package has no vulnerabilities, is secure or is unaffected. Packages without bars SHALL be described as having no records in the Wolfi security database. The Recorded fixes section SHALL keep its note that this data doesn't list unfixed vulnerabilities, and SHALL point to the Open advisories section for those.

#### Scenario: SF-7.1 Packages without records
- **WHEN** the chart is shown for `python` `latest`
- **THEN** a line reads "9 more source packages have no records in the Wolfi security database."

#### Scenario: SF-7.2 No package has records
- **WHEN** an image's source packages all have no records
- **THEN** no chart is drawn and the text says none of its source packages have records in the Wolfi security database
- **AND** no text on the tab says the image has no vulnerabilities or is secure

#### Scenario: SF-7.3 Note stays
- **WHEN** the chart is shown
- **THEN** the Recorded fixes section says the Wolfi security database doesn't list unfixed vulnerabilities and isn't a full scan
- **AND** it refers to the Open advisories section for known open advisories

### Requirement: SF-8 Data, tag changes and unavailable images
The fixes chart SHALL be computed from the per-package summary the Security tab already loads, with no additional API request for the chart. It SHALL recalculate when the tag changes, and SHALL NOT be shown while the summary is loading or when it fails, including for subscription-only images. The tab's separate Open advisories section makes its own request (OA-13) and is not part of the chart's data.

#### Scenario: SF-8.1 No extra requests
- **WHEN** the chart is shown
- **THEN** no API request is made for the chart beyond the vulnerability summary request the tab already makes
- **AND** the only other request the tab makes for this image's data is the Open advisories section's own

#### Scenario: SF-8.2 Switching tag
- **WHEN** the user switches `python` from `latest` to `latest-dev`
- **THEN** the chart shows the 25 bars of `latest-dev`, starting with `openssl-4.0`, `python-3.14` and `curl`

#### Scenario: SF-8.3 Subscription-only image
- **WHEN** the Security tab is opened for a subscription-only image such as `loki-fips`
- **THEN** the existing subscription message is shown and no chart is drawn
