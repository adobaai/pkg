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

func TestParseSignedOrders(t *testing.T) {
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
			name:   "SinglePlusPrefix",
			input:  []string{"+foo"},
			output: []string{"foo ASC"},
		},
		{
			name:   "SingleMinusPrefix",
			input:  []string{"-bar"},
			output: []string{"bar DESC"},
		},
		{
			name:   "NoPrefix",
			input:  []string{"baz"},
			output: []string{"baz"},
		},
		{
			name:   "MixedPrefixes",
			input:  []string{"+foo", "-bar", "baz"},
			output: []string{"foo ASC", "bar DESC", "baz"},
		},
		{
			name:   "UnknownPrefix",
			input:  []string{"*qux"},
			output: []string{"*qux"},
		},
		{
			name:   "SingleCharacterField",
			input:  []string{"+a", "-b"},
			output: []string{"a ASC", "b DESC"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSignedOrders(tt.input...)
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

	t.Run("WithSignedOrders", func(t *testing.T) {
		lp := &fakeListParams{orders: []string{"existing"}}
		got := WithSignedOrders(lp, "+foo", "-bar", "baz")
		expected := []string{"foo ASC", "bar DESC", "baz"}
		assert.Equal(t, expected, got.GetOrders())
	})

	t.Run("ParseSignedOrdersParams", func(t *testing.T) {
		lp := &fakeListParams{orders: []string{"+foo", "-bar", "baz"}}
		got := ParseSignedOrdersParams(lp)
		expected := []string{"foo ASC", "bar DESC", "baz"}
		assert.Equal(t, expected, got.GetOrders())
	})
}
