# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic
Versioning](http://semver.org/spec/v2.0.0.html).

## Unreleased

### Added
- Added support for mTLS authentication

### Changed
- Fixed http-json example outputs

## [0.3.0] - 2021-01-20

### BREAKING CHANGE
- Changed from --path using gojsonq to --query using gojq

## [0.2.0] - 2021-01-13

### Changed
- Fixed the ordering for some assert.Equal test cases

### Added
- Added support for custom headers to all three checks

## [0.1.3] - 2020-12-17

### Changed
- README changes
- Fix some returns for state to use nil instead of err so the help is not printed

## [0.1.2] - 2020-07-31

### Changed
- README changes

## [0.1.1] - 2020-07-31

### Changed
- Fixed goreleaser

## [0.1.0] - 2020-07-31

### Added
- Initial release
