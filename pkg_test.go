package pkg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlice(t *testing.T) {
	var a Slice[int]
	a.Append(1, 2, 3)
	assert.Equal(t, []int{1, 2, 3}, a.Get())

	// The Slice type is used to address the following issue:
	var b []int
	b = append(b, 4, 5, 6)
	var c []int = b
	c = append(c, 7, 8, 9)
	assert.Equal(t, []int{4, 5, 6}, b)
	assert.Equal(t, []int{4, 5, 6, 7, 8, 9}, c)
}
