package collections

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilter(t *testing.T) {
	list := []int{1, 2, 3, 4, 5, 6}
	even := Filter(list, func(it int) bool {
		return it%2 == 0
	})
	assert.Equal(t, []int{2, 4, 6}, even)
}

func TestMap(t *testing.T) {
	ids := []int{1, 2, 3, 4}
	idStrs := Map(ids, func(it int) string { return strconv.Itoa(it) })
	assert.Equal(t, []string{"1", "2", "3", "4"}, idStrs)
}
