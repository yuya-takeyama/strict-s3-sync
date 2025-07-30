# super-s3-sync Design Document

## Overview

This document outlines the design principles and architecture of super-s3-sync, focusing on the multi-phase plan generation approach that ensures reliability, testability, and extensibility across different sync scenarios.

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

## Architecture

### Data Structures

```go
// Generic metadata for any file/object
type ItemMetadata struct {
    Path      string    // Relative path/key
    Size      int64     
    ModTime   time.Time // Optional, depending on source
    Checksum  string    // May be empty initially
}

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

### Phase Functions

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
    SourceType_FileSystem,  // Will calculate SHA-256 from files
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
    DestType_FileSystem)    // Will calculate SHA-256 from files

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

### Incremental Sync

The phase separation makes it easy to implement incremental sync:
- Store Phase 1 results with timestamps
- Only reprocess changed items
- Merge with previous results

### Bidirectional Sync

The symmetric design allows for bidirectional sync:
- Run Phase 1 in both directions
- Resolve conflicts based on timestamps or policies
- Generate plans for both directions

### Extended Metadata

The design can accommodate additional metadata:
- Permissions and ownership
- Extended attributes
- Custom headers or tags

## Conclusion

This multi-phase design provides a solid foundation for a reliable, testable, and extensible sync tool. By separating I/O from pure logic and breaking the process into distinct phases, we achieve better maintainability and clearer reasoning about the sync behavior.