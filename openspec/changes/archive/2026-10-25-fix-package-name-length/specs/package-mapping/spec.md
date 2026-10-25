# Spec Delta

## MODIFIED Requirements

### Requirement: PM-9 Input validation
A request SHALL be refused as invalid input when it has no names, more than 100 names, a name longer than 128 characters, or a name that isn't a valid Debian package name (lowercase letters, digits and `+`, `-` or `.`, starting with a letter or digit, at least 2 characters). The error SHALL name the first offending value, or the start of it when it is too long.

#### Scenario: PM-9.1 Invalid name
- **WHEN** the names `curl` and `Curl;rm` are mapped
- **THEN** the request is refused as invalid input naming `Curl;rm`

#### Scenario: PM-9.2 Too many names
- **WHEN** 101 names are mapped
- **THEN** the request is refused as invalid input

#### Scenario: PM-9.3 No names
- **WHEN** an empty list is mapped
- **THEN** the request is refused as invalid input

#### Scenario: PM-9.5 Name too long
- **WHEN** a valid-looking name of 129 characters is mapped
- **THEN** the request is refused as invalid input, while a name of 128 characters is mapped
