# Optimizations Backlog

## Current Known Bottleneck

- ONNX vector generation in indexing is currently O(N) sequential per chunk.
- Current MVP prioritizes correctness and stability over parallel throughput.

## Implemented MVP Optimization (2026-04-12)

- Vector DB persistence now supports single-transaction batch upserts for chunk vectors and embedding hashes.
- Indexer emits timing metrics for embedding time, DB write time, and total topic indexing time.

## Next Upgrade Path

- Add an `errgroup`-based worker pool for embedding generation with bounded concurrency.
- Add dynamic batching strategy tuned to model/token limits.
- Keep writes transactional per topic batch to preserve integrity and limit sync overhead.
