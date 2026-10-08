//go:build appengine && go1.26
// +build appengine,go1.26

package fwd

func unsafestr(s string) []byte { return []byte(s) }
