package dbz

const MaxLimit = 100

// For represents explicit row locking.
//
// See https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-ROWS
type For int8

const (
	Nothing For = iota
	Update
	Share
)

// ListParams is for offset-limit pagination
// This is mainly used with proto, so we use uint32.
type ListParams interface {
	GetLimit() uint32
	GetOffset() uint32
	GetOrders() []string
}

// BaseList is a basic implementation of ListParams.
// It provides default values for limit, offset, and orders.
type BaseList struct {
	Limit    uint32
	Offset   uint32
	Orders   []string
	MaxLimit int32
}

func (p *BaseList) GetLimit() uint32 {
	if p == nil {
		return MaxLimit
	}
	return p.Limit

}

func (p *BaseList) GetOffset() uint32 {
	if p == nil {
		return 0
	}
	return p.Offset
}

func (p *BaseList) GetOrders() []string {
	if p == nil {
		return nil
	}
	return p.Orders
}

// SafeLimit ensures that the limit does not exceed the maximum allowed limit.
func SafeLimit(limit uint32, max ...int32) uint32 {
	max = append(max, MaxLimit)
	m := max[0]
	switch {
	case m == -1:
		return limit
	case limit == 0, limit > uint32(m):
		return uint32(m)
	default:
		return limit
	}
}

// SafeList returns a ListParams with a safe limit and the same offset and orders.
func SafeList(p ListParams) ListParams {
	var maxs []int32
	if bp, ok := p.(*BaseList); ok {
		if m := bp.MaxLimit; m != 0 {
			maxs = append(maxs, m)
		}
	}
	return &BaseList{
		Limit:  SafeLimit(p.GetLimit(), maxs...),
		Offset: p.GetOffset(),
		Orders: p.GetOrders(),
	}
}

// WithLimit overrides the limit in ListParams.
func WithOffset(lp ListParams, offset uint32) ListParams {
	if bl, ok := lp.(*BaseList); !ok {
		return &BaseList{
			Limit:  lp.GetLimit(),
			Offset: offset,
			Orders: lp.GetOrders(),
		}
	} else {
		bl.Offset = offset
		return bl
	}
}

// WithLimit overrides the limit in ListParams, ensuring it does not exceed the maximum limit.
func WithLimit(lp ListParams, limit uint32, max ...int32) ListParams {
	if bl, ok := lp.(*BaseList); !ok {
		return &BaseList{
			Limit:  SafeLimit(limit, max...),
			Offset: lp.GetOffset(),
			Orders: lp.GetOrders(),
		}
	} else {
		bl.Limit = SafeLimit(limit, max...)
		return bl
	}
}

// WithOrders overrides the orders in ListParams.
func WithOrders(lp ListParams, orders ...string) ListParams {
	if bl, ok := lp.(*BaseList); !ok {
		return &BaseList{
			Limit:  lp.GetLimit(),
			Offset: lp.GetOffset(),
			Orders: orders,
		}
	} else {
		bl.Orders = orders
		return bl
	}
}

// AppendOrders appends orders to the existing orders in ListParams.
func AppendOrders(lp ListParams, orders ...string) ListParams {
	if bl, ok := lp.(*BaseList); !ok {
		return &BaseList{
			Limit:  lp.GetLimit(),
			Offset: lp.GetOffset(),
			Orders: append(lp.GetOrders(), orders...),
		}
	} else {
		bl.Orders = append(bl.Orders, orders...)
		return bl
	}
}
