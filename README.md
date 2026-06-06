# Cascade - Build a Toy LSM Tree From Scratch
by [Anirudh Rowjee](https://rowjee.com), Software Engineer - Storage @ Couchbase

## What is this?

We will go in depth to understand how a Log-Structured Merge Tree works — the storage engine architecture behind databases like RocksDB, LevelDB, Cassandra, and Couchbase. By the end of this workshop, you will have written your own LSM Tree in Go that can handle writes, reads, and deletions, persist data to disk in SSTable format, and run a full tiered compaction pass across levels.

We define read amplification as the number of Read IO Calls needed for one read operation.

The central tension we will explore: **what are you paying for reads when writes are relatively cheap?** Every design decision in an LSM Tree is a response to that question. You will feel this tradeoff directly, through a read IO counter that tracks how many IOs a read requires as data accumulates across levels.

## Who is it for?

Engineers with moderate systems programming experience who want to understand how LSM Trees work internally — and why they are designed the way they are. No prior database internals knowledge is required, but comfort with a systems programming language is expected. Basic comfort with File IO and goroutines is helpful but not a prerequisite.

## Format

~4 hours. Hands-on.

## What will we build?

A working LSM Tree in Go with three on-disk levels — an unsorted L0, and two sorted levels (L1, L2). The engine will support:

- `GET`, `UPSERT`, and `DELETE` (with tombstones)
- Flushing to disk in SSTable format
- 1KB memtable with Memtable swap
- Reads across all three levels (Amplification factor of 10: L0 10KB, L1 100KB, L2: 1MB)
- Checkpointing and recovery
- Tiered compaction across levels

We will also instrument the engine with a read IO counter, so that the impact of compaction and our read path optimisations on read amplification is directly observable.

## Stages

1. **Setup** — Get the skeleton repository running, understand the testing harness, and run the first milestone tests. Sets the feedback loop for the rest of the workshop.
2. **Flush** — Persist data to disk as an SSTable. Introduce the on-disk format and discuss encoding choices.
3. **Reads Across Levels** — Implement reads that fan out across L0, L1, and L2. Introduce the read IO counter. See directly what a read costs when data is scattered across levels.
4. **Checkpointing and Recovery** — Handle restarts gracefully: checkpoint current state and recover it on startup.
5. **Compaction in Isolation** — Understand the compaction algorithm as a standalone problem: merging and sorting two SSTable streams, handling tombstones, producing a new SSTable.
6. **Tiered Compaction End-to-End** — Wire compaction into the engine. Implement a full tiered compaction pass across L0, L1, and L2. Observe the reduction in average read IOs as compaction runs.

## Outcome

- The write-optimised tradeoff at the heart of LSM Trees, and why it exists
- The on-disk layout of an SSTable and how it is written and read
- Why L0 is unsorted, and what that costs on the read path
- How tombstones work and why deletes are non-trivial
- How reads degrade as SSTables accumulate, and how compaction recovers them
- The tiered compaction algorithm: what it costs in write amplification, what it saves in implementation complexity
- What real systems like LevelDB and RocksDB extend from this foundation

## Tooling

Go, with third-party libraries for the in-memory map and disk encoding. A skeleton repository with sample data, a load generation harness, and per-milestone tests will be provided.

## Further Reading

- **O'Neill et al. (1996)** — [The Log-Structured Merge-Tree](https://www.cs.umb.edu/~poneil/lsmtree.pdf) — the canonical paper that introduced LSM Trees. Read this after the workshop to see how much of what you built was already in the original design.
- **LevelDB source code** — [github.com/google/leveldb](https://github.com/google/leveldb) — a direct, readable implementation of the ideas you just built, and the direct ancestor of RocksDB. Every component will look familiar.
- **RocksDB Wiki: Compaction** — [github.com/facebook/rocksdb/wiki/Compaction](https://github.com/facebook/rocksdb/wiki/Compaction) — how a production system extends tiered and leveled compaction, and the operational tradeoffs between them.