// Package pkg provides some basic utilities.
package pkg

// Slice is a generic slice type that allows operations on slices via pointers.
type Slice[T any] []T

func (a *Slice[T]) Append(elems ...T) {
	*a = append(*a, elems...)
}

func (a *Slice[T]) Get() []T {
	return *a
}
