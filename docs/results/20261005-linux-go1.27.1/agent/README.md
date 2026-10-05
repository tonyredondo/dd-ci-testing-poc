# Gin runtime with Agent delivery and CI telemetry

Supplementary control: three repetitions after one warmup, all original Gin package test binaries.
CI events and telemetry use the same local HTTP Agent proxy. No API key or external fallback endpoint.
Agent delivery is uncompressed; the complete agentless matrix separately measures gzip.
This smaller control measures telemetry-enabled execution and retains its full raw request bodies and clocks.

| Project | CPUs | Native | Orchestrion | POC SDK | POC Mini |
| --- | ---: | ---: | ---: | ---: | ---: |
| Gin | 4 | 0.175121 s | 0.222531 s | 0.224727 s (+1.0%; +28.3%) | 0.201225 s (-9.6%; +14.9%) |
| Gin | 32 | 0.196113 s | 0.233637 s | 0.243795 s (+4.3%; +24.3%) | 0.222311 s (-4.8%; +13.4%) |

Three repetitions have limited precision. Read the recorded ranges and CPU totals before interpreting small changes.
