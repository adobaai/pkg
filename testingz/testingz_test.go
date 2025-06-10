package testingz

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func getUID() (int, error) {
	return 1, nil
}

func TestResult(t *testing.T) {
	v := R(1, nil).NoError(t).Equal(1).V()
	assert.Equal(t, 1, v)

	R(2, nil).NoError(t).Do(func(t *testing.T, it int) {
		assert.Equal(t, 2, it)
	})

	uid := R(getUID()).RequireV(t, "it should be 3")
	t.Log(uid)

	// R(3, nil).NoError(t).Equal(6, "it should be %d", 6)
}
