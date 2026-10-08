//go:build tinygo && go1.26
// +build tinygo,go1.26

package fwd

import (
	"unsafe"
)

// unsafe cast string as []byte
func unsafestr(b string) []byte {
	return unsafe.Slice(unsafe.StringData(b), len(b))
}
