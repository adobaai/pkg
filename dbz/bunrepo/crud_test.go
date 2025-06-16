package bunrepo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeListParams struct {
	orders []string
}

// Minimal implementation for testing
func (f *fakeListParams) GetOrders() []string { return f.orders }
func (f *fakeListParams) GetLimit() uint32    { return 0 }
func (f *fakeListParams) GetOffset() uint32   { return 0 }

func TestColonOrders(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		output []string
	}{
		{
			name:   "Nil",
			input:  nil,
			output: nil,
		},
		{
			name:   "Empty",
			input:  []string{},
			output: []string{},
		},
		{
			name:   "Single",
			input:  []string{"foo:asc"},
			output: []string{"foo ASC"},
		},
		{
			name:   "NoSuffix",
			input:  []string{"baz"},
			output: []string{"baz"},
		},
		{
			name:   "Multiple",
			input:  []string{"foo:asc", "bar:desc", "baz"},
			output: []string{"foo ASC", "bar DESC", "baz"},
		},
		{
			name:   "UnknownPrefix",
			input:  []string{"*qux"},
			output: []string{"*qux"},
		},
		{
			name:   "Complex",
			input:  []string{"a:asc:nulls:first", "b"},
			output: []string{"a ASC NULLS FIRST", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseColonOrders(tt.input...)
			if len(got) != len(tt.output) {
				t.Fatalf("expected %d results, got %d", len(tt.output), len(got))
			}
			for i := range got {
				if got[i] != tt.output[i] {
					t.Errorf("at %d: expected %q, got %q", i, tt.output[i], got[i])
				}
			}
		})
	}

	t.Run("WithColonOrders", func(t *testing.T) {
		lp := &fakeListParams{orders: []string{"existing"}}
		got := WithColonOrders(lp, "foo:asc", "bar:desc", "baz")
		expected := []string{"foo ASC", "bar DESC", "baz"}
		assert.Equal(t, expected, got.GetOrders())
	})

	t.Run("ParseColonOrdersParams", func(t *testing.T) {
		lp := &fakeListParams{orders: []string{"foo:asc", "bar:DESC", "baz"}}
		got := ParseColonOrdersParams(lp)
		expected := []string{"foo ASC", "bar DESC", "baz"}
		assert.Equal(t, expected, got.GetOrders())
	})
}
