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

type BaseList struct {
	Limit    uint32
	Offset   uint32
	Orders   []string
	MaxLimit int32
}

func (p *BaseList) GetLimit() uint32 { return p.Limit }

func (p *BaseList) GetOffset() uint32 { return p.Offset }

func (p *BaseList) GetOrders() []string { return p.Orders }

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

func SafeList(p ListParams) ListParams {
	var maxLimit []int32
	if bp, ok := p.(*BaseList); ok {
		if ml := bp.MaxLimit; ml != 0 {
			maxLimit = append(maxLimit, ml)
		}
	}
	return &BaseList{
		Limit:  SafeLimit(p.GetLimit(), maxLimit...),
		Offset: p.GetOffset(),
		Orders: p.GetOrders(),
	}
}
