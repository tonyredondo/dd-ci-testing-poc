# Native CI event client

This is POC-owned code, separate from the SDK extraction. The CI wire schema
is adapted from `ddtrace/tracer/civisibility_tslv.go` at the base recorded in
[`../thirdparty/dd-trace-go/SOURCE.json`](../thirdparty/dd-trace-go/SOURCE.json).
Its original Apache-2.0 license and notices are retained there.

| File | Responsibility |
| --- | --- |
| `client.go` | Explicit configuration, immutable common metadata, client construction |
| `batch.go` | Bounded queue, byte accounting, serialization, flush/close and error state |
| `runtime.go` | Environment configuration and the process-wide client used by testing hooks |
| `span.go` | Mutable event fields until Finish, CI identity and read-only final maps |
| `events.go` | CI-only wire schema; generated serialization lives in `events_msgp.go` |

`Finish` transfers ownership of the private metadata/metric maps once; setters
cannot change them afterward. `sendMu` serializes delivery and owns the reusable
payload buffer. `mu` protects queue/error state; failed delivery keeps the older
batch and rejects new work when bounds would be exceeded. `Close` prevents new
events before the final flush. Readers are sealed by `citransport` before buffers
or compressors are reused. Keep these invariants together when changing code;
the ownership, failed-delivery and race tests cover the consuming paths.

No APM sampler, security, profiling or remote configuration is hosted here.
An explicit client does not inherit process-global environment configuration.
Changing this code does not update upstream copies automatically; see the
[shared source update procedure](../thirdparty/README.md).
