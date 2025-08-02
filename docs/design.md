# strict-s3-sync Design Document

## Overview

This document outlines the design principles and architecture of strict-s3-sync, a strict S3 synchronization tool that uses CRC64NVME checksums for accurate file comparison.

### Why CRC64NVME?

- **Content-based verification**: Unlike timestamp-based sync, CRC64NVME ensures files are identical by comparing actual content
- **S3 native support**: S3 supports ChecksumCRC64NVME natively, avoiding the need for custom metadata
- **Reliability**: Robust checksum algorithm prevents false matches and ensures data integrity
- **Performance**: Hardware-accelerated computation for faster checksum calculation
- **Full object checksums**: Avoids composite checksum issues with multipart uploads, ensuring consistent comparisons

### Key Goals

1. **Accuracy**: Never miss a changed file due to timestamp issues
2. **Performance**: Concurrent operations with configurable parallelism
3. **Simplicity**: Clear separation between planning and execution
4. **Extensibility**: Easy to add new source/destination types

## Public API

### Planner Interface

The core abstraction for plan generation is the `Planner` interface:

```go
type Planner interface {
    Plan(ctx context.Context, source Source, dest Destination, opts Options) ([]Item, error)
}

// Source/Destination types
type SourceType string
type DestType string

const (
    SourceTypeFileSystem SourceType = "filesystem"
    SourceTypeS3        SourceType = "s3"
    
    DestTypeFileSystem  DestType = "filesystem"
    DestTypeS3         DestType = "s3"
)

// Metadata for a file or object
type ItemMetadata struct {
    Path      string    // Relative path/key
    Size      int64     
    ModTime   time.Time // Optional, depending on source
    Checksum  string    // May be empty initially
}

// Source represents where files come from
type Source struct {
    Type     SourceType  // "filesystem" or "s3"
    Path     string      // File path or S3 URI (s3://bucket/prefix)
    Metadata []ItemMetadata
}

// Destination represents where files go to
type Destination struct {
    Type     DestType    // "filesystem" or "s3"
    Path     string      // File path or S3 URI
    Metadata []ItemMetadata
}

// Options for plan generation
type Options struct {
    DeleteEnabled bool
    Excludes      []string      // Glob patterns like "*.tmp", ".git/**"
    Logger        PlanLogger    // Injectable logger for phase progress
}

// Action types
type Action string

const (
    ActionUpload Action = "upload"    // Transfer from source to destination
    ActionDelete Action = "delete"    // Remove from destination (when DeleteEnabled)
    ActionSkip   Action = "skip"      // No action needed (internal use)
)

// Plan item represents a single action to be taken
type Item struct {
    Action    Action  
    LocalPath string  // Full path for uploads (filesystem source)
    S3Key     string  // S3 object key
    Size      int64   // File size in bytes
    Reason    string  // Human-readable explanation (e.g., "new file", "checksum differs")
}
```

### Logging Interface

The planner accepts an injectable logger for monitoring progress:

```go
type PlanLogger interface {
    // Called at the start of each phase
    PhaseStart(phase string, totalItems int)
    
    // Called for each item processed
    ItemProcessed(phase string, item string, action string)
    
    // Called at the end of each phase
    PhaseComplete(phase string, processedItems int)
}

// Built-in implementations
type VerboseLogger struct{}  // Logs every file and action
type NullLogger struct{}     // No output (for tests)
// Future: ProgressLogger     // Shows percentage progress only
```

## Planner Implementations

### FSToS3Planner

The current implementation for file system to S3 synchronization:

```go
type FSToS3Planner struct {
    client *s3client.Client
    logger PlanLogger
}

func NewFSToS3Planner(client *s3client.Client, logger PlanLogger) *FSToS3Planner {
    if logger == nil {
        logger = &NullLogger{}
    }
    return &FSToS3Planner{
        client: client,
        logger: logger,
    }
}

func (p *FSToS3Planner) Plan(ctx context.Context, source Source, dest Destination, opts Options) ([]Item, error) {
    // Implementation uses internal phases, but this is hidden from API users
    // See Implementation Details section below
}
```

Usage example:

```go
// Create planner with verbose logging
logger := &VerboseLogger{}
planner := NewFSToS3Planner(s3Client, logger)

// Generate plan
plan, err := planner.Plan(ctx, localSource, s3Dest, Options{
    DeleteEnabled: true,
    Excludes:      []string{"*.tmp", ".git/**"},
    Logger:        logger,
})

// Execute plan (separate from planning)
executor := NewExecutor(s3Client)
results := executor.Execute(ctx, plan)
```

## Core Design Principles

### 1. Separation of I/O and Pure Logic

The sync process is divided into distinct phases, separating I/O operations from pure computational logic. This separation enables:

- **Testability**: Pure functions can be tested without mocks or external dependencies
- **Reproducibility**: Given the same input state, the plan generation always produces the same output
- **Debuggability**: Each phase can be inspected and validated independently

### 2. Multi-Phase Plan Generation

The sync planning process is explicitly divided into three phases:

```
Phase 1: Metadata Comparison (Pure Function)
├── Input: Source and destination metadata
├── Output: Initial classification of items
└── Logic: Compare existence and size

Phase 2: Checksum Collection (I/O)
├── Input: Items requiring checksum verification
├── Output: Checksum data for relevant items
└── Logic: Retrieve or calculate checksums as needed

Phase 3: Final Plan Generation (Pure Function)
├── Input: Phase 1 results + Phase 2 checksums
├── Output: Final sync plan with actions
└── Logic: Merge results and generate actions
```

### 3. Immutable State Representation

All state collected from I/O operations is represented as immutable data structures, enabling:

- Safe concurrent processing
- Clear data flow between phases
- Easy serialization for caching or debugging

## Implementation Details

The following sections describe the internal architecture of the planners. These details are not exposed in the public API but are documented here for maintainers and contributors.

### Internal Architecture

#### Data Structures

```go
// Phase 1 output
type Phase1Result struct {
    NewItems      []ItemRef    // Items only in source
    DeletedItems  []ItemRef    // Items only in destination
    SizeMismatch  []ItemRef    // Different sizes
    NeedChecksum  []ItemRef    // Same size, need checksum verification
    Identical     []ItemRef    // Already confirmed identical
}

// Phase 2 output
type ChecksumData struct {
    ItemRef         ItemRef
    SourceChecksum  string
    DestChecksum    string
}

// Final plan item
type PlanItem struct {
    Action   Action  // Upload, Download, Delete, Skip
    Source   string  // Source path/key
    Dest     string  // Destination path/key
    Size     int64
    Reason   string  // Human-readable explanation
}
```

#### Internal Phase Functions

```go
// Pure function - no I/O
func Phase1Compare(
    source []ItemMetadata,
    dest []ItemMetadata,
    deleteEnabled bool,
) Phase1Result

// I/O function - retrieves/calculates checksums
func Phase2CollectChecksums(
    ctx context.Context,
    items []ItemRef,
    sourceType SourceType,
    destType DestType,
) ([]ChecksumData, error)

// Pure function - no I/O
func Phase3GeneratePlan(
    phase1 Phase1Result,
    checksums []ChecksumData,
) []PlanItem
```

## Implementation Examples

### File System to S3

```go
// I/O: Collect local file metadata
localFiles := walker.Walk(localPath)

// I/O: Collect S3 object metadata
s3Objects := s3client.ListObjects(bucket, prefix)

// Phase 1: Pure comparison
phase1Result := Phase1Compare(localFiles, s3Objects, deleteEnabled)

// Phase 2: I/O for checksums
checksums := Phase2CollectChecksums(ctx, phase1Result.NeedChecksum, 
    SourceType_FileSystem,  // Will calculate CRC64NVME from files
    DestType_S3)           // Will use HeadObject to get checksums

// Phase 3: Pure plan generation
finalPlan := Phase3GeneratePlan(phase1Result, checksums)

// Execution (I/O)
results := executor.Execute(finalPlan)
```

### S3 to File System

```go
// Same structure, but Phase 2 behaves differently:
checksums := Phase2CollectChecksums(ctx, phase1Result.NeedChecksum,
    SourceType_S3,          // Will use HeadObject to get checksums
    DestType_FileSystem)    // Will calculate CRC64NVME from files

// The rest remains identical
```

### S3 to S3

```go
// Both source and destination use HeadObject - no calculation needed
checksums := Phase2CollectChecksums(ctx, phase1Result.NeedChecksum,
    SourceType_S3,    // HeadObject for source bucket
    DestType_S3)      // HeadObject for destination bucket

// This is more efficient as no checksum calculation is required
```

## Benefits of This Design

### 1. Testability

Each phase can be tested independently:
- Phase 1: Test with various metadata combinations
- Phase 2: Mock I/O operations
- Phase 3: Test with different phase1/phase2 outputs

### 2. Performance Optimization

- Phase 1 is fast and can process millions of items quickly
- Phase 2 only processes items that need checksum verification
- Checksum calculation/retrieval can be parallelized with controlled concurrency

### 3. Extensibility

New sync scenarios can be added by:
1. Implementing the metadata collection for the new source/destination
2. Implementing checksum collection for the new type
3. Reusing the same phase functions

### 4. Debugging and Monitoring

- Each phase output can be logged/inspected
- Progress can be reported at phase boundaries
- Dry-run can show the plan without execution

### 5. Caching Opportunities

- Phase 1 results can be cached for large directories
- Checksums can be cached with file modification time
- Plans can be saved and re-executed

## Future Considerations

### Additional Planner Implementations

When needed, the following planners can be implemented using the same interface:

#### S3ToS3Planner
- For bucket-to-bucket synchronization
- Leverages server-side copy operations
- No local I/O required

#### S3ToFSPlanner  
- For downloading from S3 to local file system
- Inverse of current FSToS3Planner

### Logger Implementations

#### ProgressLogger
- Shows only percentage progress per phase
- Useful for large-scale operations
- Minimal output while maintaining visibility

#### JSONLogger
- Structured logging for machine processing
- Integration with monitoring systems

### Advanced Features

#### Incremental Sync
- Wrapper around base planners
- State persistence between runs
- Only process changed items

#### Parallel Planning
- Split large directories into chunks
- Generate plans concurrently
- Merge results for execution

## CLI Design

### Command Interface

The CLI follows the same interface as `aws s3 sync` for familiarity and ease of adoption:

```bash
strict-s3-sync <LocalPath> <S3Uri> [options]
```

### Core Options (MVP)

These options are implemented first to match essential `aws s3 sync` functionality:

| Option | Description | AWS Compatibility |
|--------|-------------|-------------------|
| `--dryrun` | Shows operations without executing | Same as `aws s3 sync` |
| `--delete` | Delete dest files not in source | Same as `aws s3 sync` |
| `--exclude <pattern>` | Exclude patterns (multiple allowed) | Same as `aws s3 sync` |
| `--include <pattern>` | Include patterns (multiple allowed) | Same as `aws s3 sync` |
| `--quiet` | Suppress non-error output | Similar to `aws s3 sync` |

### Additional Options

Options specific to strict-s3-sync:

| Option | Description | Default |
|--------|-------------|---------|
| `--concurrency <n>` | Number of concurrent operations | 32 |
| `--skip-missing-checksum` | Skip files without S3 checksums | false |

### Option Naming Philosophy

- Follow `aws s3 sync` conventions exactly (e.g., `--dryrun` not `--dry-run`)
- This ensures muscle memory works for AWS CLI users
- Document any deviations clearly

### Examples

```bash
# Basic sync
strict-s3-sync ./local s3://bucket/prefix/

# Preview changes
strict-s3-sync ./local s3://bucket/prefix/ --dryrun

# Sync with deletion
strict-s3-sync ./local s3://bucket/prefix/ --delete

# Complex exclusions
strict-s3-sync ./local s3://bucket/prefix/ \
  --exclude "*.tmp" \
  --exclude ".git/*" \
  --exclude "node_modules/**"
```

### Future CLI Options

To maintain compatibility with `aws s3 sync`, these may be added later:

- `--size-only`: Skip checksum comparison
- `--exact-timestamps`: Use exact timestamp comparison
- `--no-progress`: Disable progress output
- `--storage-class`: Set S3 storage class
- `--sse`: Server-side encryption options
- `--acl`: Access control list settings

## Implementation Guide

### Dependencies

```go
// Required external dependencies
github.com/aws/aws-sdk-go-v2/config      // AWS configuration
github.com/aws/aws-sdk-go-v2/service/s3  // S3 operations
github.com/bmatcuk/doublestar/v4         // Glob pattern matching
github.com/spf13/cobra                   // CLI framework (optional)
```

### Error Handling

1. **Network errors**: Implement exponential backoff with jitter
2. **File system errors**: Log and continue with other files
3. **Permission errors**: Fail fast with clear error messages
4. **Checksum mismatches**: Always favor re-upload over skip

### Implementation Steps

1. **Create the Planner interface and types** (as defined above)
2. **Implement VerboseLogger and NullLogger**
3. **Create FSToS3Planner with three internal phases**:
   - `gatherMetadata()`: Collect file lists from source and destination
   - `verifyChecksums()`: Compare checksums for same-sized files
   - `generatePlan()`: Create final action items
4. **Implement concurrent checksum calculation** with worker pool
5. **Add unit tests** using NullLogger and mock data
6. **Create CLI wrapper** for command-line usage

### Testing Strategy

```go
// Example test structure
func TestFSToS3Planner_Plan(t *testing.T) {
    // Create test data
    source := Source{
        Type: "filesystem",
        Path: "/test/path",
        Metadata: []ItemMetadata{
            {Path: "file1.txt", Size: 100},
            {Path: "file2.txt", Size: 200},
        },
    }
    
    // Test with NullLogger
    planner := NewFSToS3Planner(mockS3Client, &NullLogger{})
    items, err := planner.Plan(ctx, source, dest, opts)
    
    // Verify plan correctness
    assert.NoError(t, err)
    assert.Len(t, items, 2)
}
```

## Conclusion

This multi-phase design provides a solid foundation for a reliable, testable, and extensible sync tool. By separating I/O from pure logic and breaking the process into distinct phases, we achieve better maintainability and clearer reasoning about the sync behavior.