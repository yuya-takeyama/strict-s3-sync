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
- `--plan-json-file <path>`: Output execution plan to a JSON file
- `--result-json-file <path>`: Output execution results to a JSON file (not generated in dry-run mode)

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

Generate JSON reports for CI/CD:

```bash
# Output both plan and result
strict-s3-sync ./local-folder s3://my-bucket/prefix/ --plan-json-file plan.json --result-json-file result.json

# Output only plan (useful with dry-run)
strict-s3-sync ./local-folder s3://my-bucket/prefix/ --dryrun --plan-json-file plan.json
```

## JSON Output Formats

### Plan JSON (`--plan-json-file`)

Outputs the execution plan before any operations:

```json
{
  "files": [
    {
      "action": "create",
      "source": "/Users/yuya/project/file1.txt",
      "target": "s3://my-bucket/prefix/file1.txt",
      "reason": "new file"
    },
    {
      "action": "update",
      "source": "/Users/yuya/project/file2.txt",
      "target": "s3://my-bucket/prefix/file2.txt",
      "reason": "checksum differs"
    },
    {
      "action": "skip",
      "source": "/Users/yuya/project/file3.txt",
      "target": "s3://my-bucket/prefix/file3.txt",
      "reason": "unchanged"
    },
    {
      "action": "delete",
      "target": "s3://my-bucket/prefix/old-file.txt",
      "reason": "deleted locally"
    }
  ],
  "summary": {
    "skip": 1,
    "create": 1,
    "update": 1,
    "delete": 1
  }
}
```

Plan actions: `skip`, `create`, `update`, `delete` (present tense)

### Result JSON (`--result-json-file`)

Outputs actual execution results (not generated in dry-run mode):

```json
{
  "files": [
    {
      "action": "created",
      "source": "/Users/yuya/project/file1.txt",
      "target": "s3://my-bucket/prefix/file1.txt"
    },
    {
      "action": "updated",
      "source": "/Users/yuya/project/file2.txt",
      "target": "s3://my-bucket/prefix/file2.txt"
    },
    {
      "action": "skipped",
      "source": "/Users/yuya/project/file3.txt",
      "target": "s3://my-bucket/prefix/file3.txt"
    },
    {
      "action": "deleted",
      "target": "s3://my-bucket/prefix/old-file.txt"
    }
  ],
  "errors": [],
  "summary": {
    "skipped": 1,
    "created": 1,
    "updated": 1,
    "deleted": 1,
    "failed": 0
  }
}
```

Result actions: `skipped`, `created`, `updated`, `deleted` (past tense)

Failed operations appear in the `errors` array with error messages.

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

This project includes code derived from Python's fnmatch module (licensed under PSF-2.0).
See [LICENSE](LICENSE) and [pkg/fnmatch/LICENSE-PSF](pkg/fnmatch/LICENSE-PSF) for details.
