# strict-s3-sync

A strict S3 sync tool that uses CRC64NVME checksums for accurate synchronization. Unlike traditional sync tools that rely on timestamps or ETags, this tool ensures data integrity by comparing CRC64NVME checksums.

## Features

- **CRC64NVME based synchronization**: Uses S3's native ChecksumCRC64NVME for accurate file comparison
- **Concurrent operations**: Uploads/deletes with configurable parallelism
- **Smart sync**: Only transfers files that have actually changed
- **Exclude patterns**: Support for glob patterns (including `**` wildcards)
- **Delete synchronization**: Optionally remove files from S3 that don't exist locally
- **Dry run mode**: Preview changes before applying them

## Installation

### Using aqua (Recommended)

```bash
# Install aqua if not already installed
# https://aquaproj.github.io/

# Install strict-s3-sync
aqua g -i yuya-takeyama/strict-s3-sync
```

### Using go install

```bash
go install github.com/yuya-takeyama/strict-s3-sync/cmd/strict-s3-sync@latest
```

### Download binary

Download from [Releases](https://github.com/yuya-takeyama/strict-s3-sync/releases)

### Build from source

```bash
git clone https://github.com/yuya-takeyama/strict-s3-sync
cd strict-s3-sync
go build -o strict-s3-sync ./cmd/strict-s3-sync
```

## Usage

```bash
strict-s3-sync <LocalPath> <S3Uri> [options]
```

### Options

- `--exclude <pattern>`: Exclude patterns (can be specified multiple times)
- `--delete`: Delete files in destination that don't exist in source
- `--dryrun`: Show what would be done without actually doing it
- `--concurrency <n>`: Number of concurrent operations (default: 32)
- `--profile <profile>`: AWS profile to use
- `--region <region>`: AWS region (uses default if not specified)
- `--quiet`: Suppress output
- `--result-json-file <path>`: Output sync results to a JSON file (useful for CI/CD pipelines)

### Examples

Basic sync:

```bash
strict-s3-sync ./local-folder s3://my-bucket/prefix/
```

Sync with exclusions:

```bash
strict-s3-sync ./local-folder s3://my-bucket/prefix/ --exclude "*.tmp" --exclude "**/.git/**"
```

Sync with deletion:

```bash
strict-s3-sync ./local-folder s3://my-bucket/prefix/ --delete
```

Dry run to preview changes:

```bash
strict-s3-sync ./local-folder s3://my-bucket/prefix/ --delete --dryrun
```

Generate JSON report for CI/CD:

```bash
strict-s3-sync ./local-folder s3://my-bucket/prefix/ --result-json-file sync-result.json
```

## JSON Output Format

When using `--result-json-file`, the tool outputs a structured JSON report:

```json
{
  "changes": [
    {
      "action": "create",
      "source": "/Users/yuya/project/file1.txt",
      "target": "s3://my-bucket/prefix/file1.txt"
    },
    {
      "action": "update",
      "source": "/Users/yuya/project/file2.txt",
      "target": "s3://my-bucket/prefix/file2.txt"
    },
    {
      "action": "delete",
      "target": "s3://my-bucket/prefix/old-file.txt"
    }
  ],
  "summary": {
    "total_files": 3,
    "created": 1,
    "updated": 1,
    "deleted": 1,
    "failed": 0
  }
}
```

Actions can be: `create`, `update`, or `delete`.

## How it Works

1. **Local Scan**: Recursively scans the local directory, applying exclude patterns
2. **S3 Listing**: Lists all objects in the S3 destination
3. **Comparison**:
   - New files (not in S3) → Upload
   - Different sizes → Upload
   - Same size → Compare CRC64NVME checksums
4. **Execution**: Performs uploads/deletes in parallel

## CRC64NVME Checksum Handling

- For uploads, S3's native ChecksumCRC64NVME is used
- Files without checksums are re-uploaded by default (natural backfill)
- Hardware-accelerated for better performance

## Required AWS Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:ListBucket",
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject"
      ],
      "Resource": ["arn:aws:s3:::your-bucket", "arn:aws:s3:::your-bucket/*"]
    }
  ]
}
```

## Performance Tips

- Adjust `--concurrency` based on your network and S3 rate limits
- Use `--exclude` patterns to skip unnecessary files
- Note: Maximum file size is 5GB (AWS PutObject limit)

## License

MIT
