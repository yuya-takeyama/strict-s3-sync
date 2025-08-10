# Changelog

## [v0.3.0](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.2.0...v0.3.0) - 2025-08-10
- feat: separate plan and result JSON outputs with improved structure by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/52
- refactor: rename "action" to "result" field in result JSON by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/54

## [v0.2.0](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.1.1...v0.2.0) - 2025-08-10
- feat: AWS S3 sync compatible exclude patterns by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/50

## [v0.1.1](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.1.0...v0.1.1) - 2025-08-10
- fix: normalize S3 URI prefix to handle trailing slashes consistently by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/47
- chore: simplify checksum filename to checksums.txt by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/49

## [v0.1.0](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.0.12...v0.1.0) - 2025-08-03
- feat: add --result-json-file option for programmatic result processing by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/44

## [v0.0.12](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.0.11...v0.0.12) - 2025-08-03
- Create custom release workflow to coexist with tagpr by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/42

## [v0.0.11](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.0.10...v0.0.11) - 2025-08-03
- Remove cosign configuration for go-release-workflow v6.0.0 by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/40

## [v0.0.10](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.0.9...v0.0.10) - 2025-08-03
- Fix release workflow errors by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/38

## [v0.0.9](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.0.8...v0.0.9) - 2025-08-03
- feat: add syft for SBOM generation by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/36

## [v0.0.8](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.0.7...v0.0.8) - 2025-08-03
- chore: add third_party_licenses to .gitignore by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/34

## [v0.0.7](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.0.6...v0.0.7) - 2025-08-03
- fix: remove invalid toolchain directive from go.mod by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/32

## [v0.0.6](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.0.5...v0.0.6) - 2025-08-03
- Delete VERSION by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/25
- chore(deps): pin Songmu/tagpr action to v1.7.0 by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/27
- feat(workflows): enable manual release workflow triggering by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/28
- fix(workflows): enable GitHub App to trigger release workflow by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/29
- fix(workflows): enable GitHub App to trigger release workflow by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/30

## [v0.0.6](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.0.5...v0.0.6) - 2025-08-03
- Delete VERSION by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/25
- chore(deps): pin Songmu/tagpr action to v1.7.0 by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/27
- feat(workflows): enable manual release workflow triggering by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/28
- fix(workflows): enable GitHub App to trigger release workflow by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/29

## [v0.0.5](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.0.4...v0.0.5) - 2025-08-03
- feat(claude): add prettier hooks for YAML, JSON, and Markdown files by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/22
- feat: add development tools and migrate release workflow by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/24

## [v0.0.4](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.0.3...v0.0.4) - 2025-08-03
- feat(workflows): use GitHub App authentication for tagpr by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/20

## [v0.0.3](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.0.2...v0.0.3) - 2025-08-03
- fix: Update release workflow tag pattern to match semver by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/18

## [v0.0.2](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.0.1...v0.0.2) - 2025-08-03
- fix: Update tagpr workflow to match official documentation by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/16

## [v0.0.1](https://github.com/yuya-takeyama/strict-s3-sync/commits/v0.0.1) - 2025-08-03
- feat: implement strict S3 sync with CRC64NVME checksums and content-type detection by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/1
- feat: implement multipart upload for large files by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/4
- feat: Parallelize Phase2 checksum calculations for massive performance gains by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/5
- feat: Implement automated release pipeline with GoReleaser and tagpr by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/12
- fix: Update tagpr to v1.7.0 and add required permissions by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/13
- fix: Use version tag instead of commit hash for tagpr by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/14
