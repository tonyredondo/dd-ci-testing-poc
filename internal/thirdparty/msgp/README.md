# tinylib/msgp source subset

- Repository: https://github.com/tinylib/msgp
- Version: `v1.6.4`
- Commit: `6f99c863451752e6aa9c7aacde4215a471a64242`
- Original license: [LICENSE](LICENSE).
- File/source hashes and local adaptations: [SOURCE.json](SOURCE.json).

The runtime and original tests retain the upstream `msgp/` package subtree. Imports target the adjacent internal `fwd` runtime. The generator remains an isolated, pinned build-time tool; generated directives are relocated. Go-derived code retains its BSD notice in `LICENSE-go`. Runtime encoding behavior is unchanged.

Follow the [shared audit and update procedure](../README.md#audit-and-update).
