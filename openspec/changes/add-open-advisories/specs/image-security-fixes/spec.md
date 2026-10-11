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
