package dbz

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type customList struct {
	limit  uint32
	offset uint32
	orders []string
}

func (c *customList) GetLimit() uint32    { return c.limit }
func (c *customList) GetOffset() uint32   { return c.offset }
func (c *customList) GetOrders() []string { return c.orders }

func TestBaseLimit(t *testing.T) {
	var bl *BaseList
	assert.Equal(t, uint32(MaxLimit), bl.GetLimit())
	assert.Equal(t, uint32(0), bl.GetOffset())
	assert.Len(t, bl.GetOrders(), 0)
}

func TestWith(t *testing.T) {
	cl := &customList{limit: 20, offset: 2, orders: []string{"foo"}}

	t.Run("Offset", func(t *testing.T) {
		bl := &BaseList{Limit: 10, Offset: 5}
		res := WithOffset(bl, 15)
		assert.Equal(t, uint32(15), res.GetOffset())
		assert.Equal(t, bl.Limit, res.GetLimit())
		assert.Equal(t, bl.GetOrders(), res.GetOrders())

		res = WithOffset(cl, 20)
		assert.Equal(t, cl.GetLimit(), res.GetLimit())
	})

	t.Run("Limit", func(t *testing.T) {
		bl := &BaseList{Limit: 10, Offset: 5}
		res := WithLimit(bl, 20)
		assert.Equal(t, uint32(20), res.GetLimit())
		assert.Equal(t, bl.GetOrders(), res.GetOrders())

		res = WithLimit(cl, 20, 10)
		assert.Equal(t, uint32(10), res.GetLimit())
		res = WithLimit(cl, 120, -10)
		assert.Equal(t, uint32(120), res.GetLimit())
		assert.Equal(t, cl.offset, res.GetOffset())
	})

	t.Run("Orders", func(t *testing.T) {
		bl := &BaseList{Limit: 10, Offset: 5, Orders: []string{"id ASC"}}
		res := WithOrders(bl, "name DESC", "created_at DESC")
		assert.Equal(t, []string{"name DESC", "created_at DESC"}, res.GetOrders())
		assert.Equal(t, bl.Limit, res.GetLimit())
		assert.Equal(t, bl.Offset, res.GetOffset())

		res = WithOrders(cl, "bar", "baz")
		assert.Equal(t, []string{"bar", "baz"}, res.GetOrders())
		assert.Equal(t, cl.limit, res.GetLimit())
	})

	t.Run("SafeList", func(t *testing.T) {
		res := SafeList(cl)
		assert.Equal(t, cl.limit, res.GetLimit())
		assert.Equal(t, cl.offset, res.GetOffset())
		assert.Equal(t, []string{"foo"}, res.GetOrders())

		bl := &BaseList{Limit: 10, Offset: 5, MaxLimit: 100}
		res = SafeList(bl)
		assert.Equal(t, uint32(10), res.GetLimit())
		assert.Equal(t, bl.Offset, res.GetOffset())
	})
}

func TestAppendOrders(t *testing.T) {
	t.Run("BaseList", func(t *testing.T) {
		bl := &BaseList{
			Limit:  10,
			Offset: 5,
			Orders: []string{"id ASC"},
		}
		res := AppendOrders(bl, "name DESC", "created_at DESC")
		want := []string{"id ASC", "name DESC", "created_at DESC"}
		assert.Equal(t, want, res.GetOrders())
		assert.Equal(t, bl, res)
	})

	t.Run("CustomList", func(t *testing.T) {
		cl := &customList{
			limit:  20,
			offset: 2,
			orders: []string{"foo"},
		}
		res := AppendOrders(cl, "bar", "baz")
		want := []string{"foo", "bar", "baz"}
		assert.Equal(t, want, res.GetOrders())
		// Should return a *BaseList, not the original customList
		if _, ok := res.(*BaseList); !ok {
			t.Errorf("expected result to be *BaseList")
		}
	})

	t.Run("EmptyOrders", func(t *testing.T) {
		bl := &BaseList{
			Limit:  1,
			Offset: 0,
		}
		res := AppendOrders(bl, "foo")
		want := []string{"foo"}
		assert.Equal(t, want, res.GetOrders())
	})

	t.Run("NoNewOrders", func(t *testing.T) {
		bl := &BaseList{
			Limit:  1,
			Offset: 0,
			Orders: []string{"foo"},
		}
		res := AppendOrders(bl)
		want := []string{"foo"}
		assert.Equal(t, want, res.GetOrders())
	})
}
