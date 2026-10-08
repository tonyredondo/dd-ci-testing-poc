// Package compat provides small standard-library equivalents that keep Mini's
// sources valid with the consumer module's Go language version.
package compat

import (
	"strings"
	"unicode/utf8"
)

// StringIterator scans substrings without allocating a slice of fields.
type StringIterator struct {
	rest, separator, value string
	fieldSeparator         func(rune) bool
	done                   bool
}

// Split has strings.Split semantics, including empty fields and UTF-8 splitting
// when the separator is empty. Next advances and Value returns the current part.
func Split(value, separator string) StringIterator {
	return StringIterator{rest: value, separator: separator}
}

// FieldsFunc scans the nonempty fields separated by predicate.
func FieldsFunc(value string, predicate func(rune) bool) StringIterator {
	return StringIterator{rest: value, fieldSeparator: predicate}
}

// Next advances to the next substring. It returns false after the final part.
func (s *StringIterator) Next() bool {
	if s.done {
		return false
	}
	if s.fieldSeparator != nil {
		start := strings.IndexFunc(s.rest, func(r rune) bool { return !s.fieldSeparator(r) })
		if start < 0 {
			s.done = true
			return false
		}
		s.rest = s.rest[start:]
		end := strings.IndexFunc(s.rest, s.fieldSeparator)
		if end < 0 {
			s.value, s.rest, s.done = s.rest, "", true
		} else {
			s.value, s.rest = s.rest[:end], s.rest[end:]
		}
		return true
	}
	if s.separator == "" {
		if s.rest == "" {
			s.done = true
			return false
		}
		_, size := utf8.DecodeRuneInString(s.rest)
		s.value, s.rest = s.rest[:size], s.rest[size:]
		return true
	}
	var found bool
	s.value, s.rest, found = strings.Cut(s.rest, s.separator)
	s.done = !found
	return true
}

// Value returns the substring selected by the last successful Next call.
func (s *StringIterator) Value() string { return s.value }
