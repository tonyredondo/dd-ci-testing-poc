# Gin with local Agent delivery and telemetry

Three measured runs follow one warmup at each CPU count. Native and Orchestrion retain their original observations; Mini uses the new revision. Real CI events and telemetry are sent through the local EVP/telemetry proxy.

| Project | Flags | CPUs | Native | Orchestrion | POC Mini |
| --- | --- | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 0.175121 s | 0.222531 s | 0.214782 s (-3.5%; +22.6%) |
| Gin | `none` | 32 | 0.196113 s | 0.233637 s | 0.210877 s (-9.7%; +7.5%) |

## Memory

| Project | Flags | CPUs | Native | Orchestrion | POC Mini |
| --- | --- | ---: | ---: | ---: | ---: |
| Gin | `none` | 4 | 41.4 MiB | 77.5 MiB | 64.8 MiB (-16.4%; +56.4%) |
| Gin | `none` | 32 | 41.5 MiB | 78.2 MiB | 72.9 MiB (-6.8%; +75.5%) |
