package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChain(t *testing.T) {
	var calls []string

	mw1 := func(next Handler[string]) Handler[string] {
		return func(ctx string) error {
			calls = append(calls, "mw1 before")
			err := next(ctx)
			calls = append(calls, "mw1 after")
			return err
		}
	}

	mw2 := func(next Handler[string]) Handler[string] {
		return func(ctx string) error {
			calls = append(calls, "mw2 before")
			err := next(ctx)
			calls = append(calls, "mw2 after")
			return err
		}
	}

	finalHandler := func(ctx string) error {
		calls = append(calls, "final handler: "+ctx)
		return nil
	}

	chained := Chain(mw1, mw2)(finalHandler)
	require.NoError(t, chained("test"))
	require.NoError(t, chained("test2"))

	expected := []string{
		"mw1 before",
		"mw2 before",
		"final handler: test",
		"mw2 after",
		"mw1 after",
		"mw1 before",
		"mw2 before",
		"final handler: test2",
		"mw2 after",
		"mw1 after",
	}

	assert.Equal(t, expected, calls)
}
