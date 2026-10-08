package compat

import (
	"errors"
	"reflect"
)

// AsType finds a matching error without depending on Go 1.26's generic helper.
func AsType[T error](err error) (T, bool) {
	var target T
	ok := errors.As(err, &target)
	return target, ok
}

// BinaryAppender is the method contract used by encoding.BinaryAppender.
type BinaryAppender interface{ AppendBinary([]byte) ([]byte, error) }

// TextAppender is the method contract used by encoding.TextAppender.
type TextAppender interface{ AppendText([]byte) ([]byte, error) }

// Pointer returns a pointer to a copy of value, including non-type expressions.
func Pointer[T any](value T) *T { return &value }

// TypeFor reports T without requiring reflect.TypeFor in an older source file.
func TypeFor[T any]() reflect.Type { return reflect.TypeOf((*T)(nil)).Elem() }

// MapSequence visits entries until yield returns false. Its callback shape is
// assignable to the standard iterator type without exposing that newer API.
func MapSequence[K comparable, V any](values map[K]V) func(func(K, V) bool) {
	return func(yield func(K, V) bool) {
		for key, value := range values {
			if !yield(key, value) {
				return
			}
		}
	}
}

// Concat returns an owned slice containing the inputs in order.
func Concat[T any](slices ...[]T) []T {
	length := 0
	for _, slice := range slices {
		length += len(slice)
		if length < 0 {
			panic("slice length overflow")
		}
	}
	var result []T
	if length != 0 {
		result = make([]T, 0, length)
	}
	for _, slice := range slices {
		result = append(result, slice...)
	}
	return result
}
