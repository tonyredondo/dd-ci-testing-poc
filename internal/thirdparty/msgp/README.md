# tinylib/msgp source subset

- Repository: https://github.com/tinylib/msgp
- Version: `v1.6.4`
- Commit: `6f99c863451752e6aa9c7aacde4215a471a64242`
- Original license: [LICENSE](LICENSE).
- File/source hashes and local adaptations: [SOURCE.json](SOURCE.json).

The runtime and tests keep the upstream `msgp/` subtree. Its buffered I/O uses
the adjacent internal `fwd` copy. Mini imports these sources without requiring
either external runtime module.

Serializers are generated with a pinned tool in an isolated module. Their
generation directives use the repository's tool wrapper. Go-derived sources
keep the BSD notice in `LICENSE-go`. Follow the
[codec update steps](../../../docs/maintenance.md#codecs-and-generated-files)
when changing sources or generation pins.

Follow the [shared audit and update procedure](../README.md#audit-and-update).

The root module keeps Go 1.21 language rules without changing the consumer.
Adapt newer syntax with ordinary loops and the small standard-library helpers
in `internal/compat`. Keep captured loop values local. Native API and feature
test version guards stay at their boundaries; avoid adding a version constraint
to every source file. Record adaptations and local hashes when updating sources.
