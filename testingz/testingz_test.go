package testingz

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResult(t *testing.T) {
	v := R(1, nil).NoError(t).Equal(1).V()
	assert.Equal(t, 1, v)

	R(2, nil).NoError(t).Do(func(t *testing.T, it int) {
		assert.Equal(t, 2, it)
	})

	// R(3, nil).NoError(t).Equal(6, "it should be %d", 6)
}
