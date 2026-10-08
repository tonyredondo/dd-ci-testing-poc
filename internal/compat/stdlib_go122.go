//go:build go1.22

package compat

import (
	"go/version"
	"math/rand/v2"
)

// CompareGoVersion uses the toolchain's complete ordering, including prereleases.
func CompareGoVersion(x, y string) int { return version.Compare(x, y) }

// RandomUint64 uses the standard process-wide pseudo-random source.
func RandomUint64() uint64 { return rand.Uint64() }

// NewChaCha8 returns the standard deterministic source used by payload tests.
func NewChaCha8(seed [32]byte) interface {
	Uint64() uint64
	Read([]byte) (int, error)
} {
	return rand.NewChaCha8(seed)
}
