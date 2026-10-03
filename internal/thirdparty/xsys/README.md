# golang/sys source subset

- Repository: https://github.com/golang/sys
- Version: `v0.47.0`
- Commit: `9e7e939dcafac07e8ab4cffa6e5fc74908413f00`
- Original license: [LICENSE](LICENSE).
- File/source hashes and local adaptations: [SOURCE.json](SOURCE.json).

Selected Windows Job Object/thread/timer functions and read-only registry operations keep the upstream `windows/` and `windows/registry/` layout. Unix kernel metadata lives in `unix/`. This is an adapted subset, not a complete x/sys copy. Fixed-buffer BSD sysctl behavior and the Solaris runtime trampoline are retained; Linux/AIX use standard-library syscalls. `EXTRACTION.json` identifies each original declaration and source hash. Local ABI, registry and metadata regression tests remain beside the implementation.

Follow the [shared audit and update procedure](../README.md#audit-and-update).
