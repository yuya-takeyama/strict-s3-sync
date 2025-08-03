# Changelog

## [v0.0.2](https://github.com/yuya-takeyama/strict-s3-sync/compare/v0.0.1...v0.0.2) - 2025-08-03
- fix: Update tagpr workflow to match official documentation by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/16

## [v0.0.1](https://github.com/yuya-takeyama/strict-s3-sync/commits/v0.0.1) - 2025-08-03
- feat: implement strict S3 sync with CRC64NVME checksums and content-type detection by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/1
- feat: implement multipart upload for large files by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/4
- feat: Parallelize Phase2 checksum calculations for massive performance gains by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/5
- feat: Implement automated release pipeline with GoReleaser and tagpr by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/12
- fix: Update tagpr to v1.7.0 and add required permissions by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/13
- fix: Use version tag instead of commit hash for tagpr by @yuya-takeyama in https://github.com/yuya-takeyama/strict-s3-sync/pull/14
