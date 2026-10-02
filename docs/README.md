# Maintainer guide

Start with [architecture](architecture.md) to understand the build overlay and
the native CI runtime. Read [maintenance](maintenance.md) before changing an
upstream copy or upgrading Go. The [performance guide](performance.md) explains
the optimizations in the code and how to measure a proposed change.

| You need to... | Read |
| --- | --- |
| Follow a test from CLI invocation to CI event delivery | [Architecture and diagrams](architecture.md) |
| Update the SDK, MessagePack codecs or platform subset | [Source updates and maintenance](maintenance.md) |
| Change allocations, batching, compression or build preparation | [Performance and ownership constraints](performance.md) |
| Configure Mini or use its public API | [Native runtime usage](mini-runtime.md) |
| Compare CI features and policy combinations with the SDK | [Feature parity and event counts](ci-parity.md) |
| Check what compatibility tests establish | [Validation contract](validation.md) |
| Compare repeated execution of the complete CI matrix | [Whole-matrix timing](ci-parity.md#repeated-whole-matrix-timing) |
| Find original timings and their limitations | [Results](results.md) and [compile-only tables](../README.md#compilation-performance) |

The source record lives beside each incorporated library in
[`internal/thirdparty`](../internal/thirdparty/README.md). Its manifests identify
the exact upstream revision, original paths, licenses and local changes.
The code in `internal/minitracer`, `internal/citransport`, `internal/runner` and
`internal/instrument` belongs to this POC.

Documentation describes the checked-in implementation. Timing artifacts describe
the revisions recorded inside them; updating the SDK or moving code does not
refresh those measurements.
