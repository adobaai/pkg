package testingz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Result provides some useful methods to write concise code in tests.
type Result[T any] struct {
	t   *testing.T
	v   T
	err error
}

func R[T any](v T, err error) *Result[T] {
	return &Result[T]{
		v:   v,
		err: err,
	}
}

func (r *Result[T]) V() T {
	return r.v
}

func (r *Result[T]) NoError(t *testing.T, msgf ...any) *Result[T] {
	require.NoError(t, r.err, msgf...)
	r.t = t
	return r
}

func (r *Result[T]) ErrorIs(t *testing.T, target error, msgf ...any) *Result[T] {
	require.ErrorIs(t, r.err, target, msgf...)
	r.t = t
	return r
}

func (r *Result[T]) ErrorAs(t *testing.T, target any, msgf ...any) *Result[T] {
	require.ErrorAs(t, r.err, target, msgf...)
	r.t = t
	return r
}

func (r *Result[T]) ErrorContains(t *testing.T, s string, msgf ...any) *Result[T] {
	require.ErrorContains(t, r.err, s, msgf...)
	r.t = t
	return r
}

func (r *Result[T]) Equal(v T, msgf ...any) *Result[T] {
	require.Equal(r.t, v, r.v, msgf...)
	return r
}

func (r *Result[T]) EqualError(t *testing.T, errStr string, msgf ...any) *Result[T] {
	require.EqualError(t, r.err, errStr, msgf...)
	return r
}

func (r *Result[T]) Log() *Result[T] {
	r.t.Log(r.v)
	return r
}

func (r *Result[T]) Nil() *Result[T] {
	require.Nil(r.t, r.v)
	return r
}

func (r *Result[T]) Do(f func(t *testing.T, it T)) *Result[T] {
	f(r.t, r.v)
	return r
}
