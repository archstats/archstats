# 0008. Concurrent Bounded File Walking

- **Status**: Accepted
- **Date**: 2026-06-01

## Context and Problem Statement

Analyzing codebases requires walking large directory trees, reading, and parsing thousands of files. Sequential single-threaded parsing is slow and does not utilize modern multi-core processors. Unbounded goroutine spawns can easily lead to resource exhaustion (e.g. "too many open files" errors) or massive memory spikes on extremely large codebases.

## Decision Drivers

- **Performance**: High throughput file parsing.
- **Resource safety**: Prevent file descriptor leaks, OS limit errors, and high memory footprints.
- **Robustness**: Graceful error handling (e.g., skip unreadable files without crashing).

## Considered Options

- **Sequential Walking**: Simple but extremely slow.
- **Unbounded Goroutines**: Fast but dangerous for large directories.
- **Bounded Worker Pool with Concurrent Channel Ingestion**: Parallel processing with a fixed concurrency cap.

## Decision Outcome

Chosen option: **Bounded Worker Pool with Concurrent Channel Ingestion** (implemented in `core/walker/walker.go`).

We use a thread-safe bounded worker pool (controlled via goroutines and channels) to read and parse files concurrently. The number of concurrent workers is bounded (e.g., matching the host CPU count or user configurations).
Unreadable or broken files do not crash the walk; they are gracefully logged/skipped.

### Consequences

- **Good**: Significant speedup in file parsing, scaling linearly with available CPU cores.
- **Good**: Strictly controlled concurrent file descriptors, avoiding OS boundary crashes.
- **Bad**: Slightly more complex concurrency orchestration and synchronization logic compared to standard sequential `filepath.Walk`.
