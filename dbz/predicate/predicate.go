package predicate

import (
	"reflect"

	"github.com/uptrace/bun" // TODO remove this dependency
)

// Op represents an operator.
//
// This is copied from ent:
// https://github.com/ent/ent/blob/cf2428d49a59ed30335d2cf10b509d4934a1c73b/dialect/sql/builder.go#L3382
type Op int

const (
	// Predicate operators.

	opEQ      Op = iota // =
	opNEQ               // <>
	opGT                // >
	opGTE               // >=
	opLT                // <
	opLTE               // <=
	opIn                // IN
	opNotIn             // NOT IN
	opLike              // LIKE
	opIsNull            // IS NULL
	opNotNull           // IS NOT NULL
)

func (o Op) String() string {
	return ops[o]
}

var ops = [...]string{
	opEQ:      "=",
	opNEQ:     "<>",
	opGT:      ">",
	opGTE:     ">=",
	opLT:      "<",
	opLTE:     "<=",
	opIn:      "IN",
	opNotIn:   "NOT IN",
	opLike:    "LIKE",
	opIsNull:  "IS NULL",
	opNotNull: "IS NOT NULL",
}

func (o Op) IsIn() bool {
	switch o {
	case opIn, opNotIn:
		return true
	default:
		return false
	}
}

func (o Op) IsUnary() bool {
	switch o {
	case opIsNull, opNotNull:
		return true
	default:
		return false
	}
}

type TokenType int8

const (
	Pred TokenType = iota
	Or
	Not
	Zero // Do not omit zero value
	Group
)

type Predicate[T any] struct {
	Op Op
	V  T // Value
	Vs []T
}

func (p Predicate[T]) IsNop() bool {
	if p.Op.IsUnary() {
		return false
	}
	if p.Op.IsIn() {
		return len(p.Vs) == 0
	}
	var v any = p.V
	if a, ok := v.(interface{ IsZero() bool }); ok {
		return a.IsZero()
	}
	return reflect.ValueOf(p.V).IsZero()
}

type Token[T any] struct {
	Type  TokenType
	Pred  Predicate[T]
	Group *Field[T]
}

// Field is a Field predicate.
type Field[T any] struct {
	Tokens []Token[T]
}

func NewField[T any]() *Field[T] {
	return new(Field[T])
}

// EQ is a shortcut of `NewFP[T]().EQ(v)`.
func EQ[T any](v T) *Field[T] { return NewField[T]().EQ(v) }

// EQZ is a shortcut of `NewFP[T]().Zero().EQ(v)`.
func EQZ[T any](v T) *Field[T] { return NewField[T]().Zero().EQ(v) }

// NEQ is a shortcut of `NewFP[T]().NEQ(v)`.
func NEQ[T any](v T) *Field[T] { return NewField[T]().NEQ(v) }

// GT is a shortcut of `NewFP[T]().GT(v)`.
func GT[T any](v T) *Field[T] { return NewField[T]().GT(v) }

// GTE is a shortcut of `NewFP[T]().GTE(v)`.
func GTE[T any](v T) *Field[T] { return NewField[T]().GTE(v) }

// LT is a shortcut of `NewFP[T]().LT(v)`.
func LT[T any](v T) *Field[T] { return NewField[T]().LT(v) }

// LTE is a shortcut of `NewFP[T]().LTE(v)`.
func LTE[T any](v T) *Field[T] { return NewField[T]().LTE(v) }

// In is a shortcut of `NewFP[T]().In(v)`.
func In[T any](v []T) *Field[T] { return NewField[T]().In(v) }

// NotIn is a shortcut of `NewFP[T]().NotIn(v)`.
func NotIn[T any](v []T) *Field[T] { return NewField[T]().NotIn(v) }

// IsNull is a shortcut of `NewFP[T]().IsNull()`.
func IsNull[T any]() *Field[T] { return NewField[T]().IsNull() }

// // Methods

// IsNop indicates whether there is no operation.
func (f *Field[T]) IsNop() bool {
	if f == nil {
		return true
	}
	for _, t := range f.Tokens {
		if !t.Pred.IsNop() || !t.Group.IsNop() {
			return false
		}
	}
	return true
}

func (f *Field[T]) append(g Token[T]) *Field[T] {
	f.Tokens = append(f.Tokens, g)
	return f
}

func (f *Field[T]) appendPred(p Predicate[T]) *Field[T] {
	return f.append(Token[T]{
		Type: Pred,
		Pred: p,
	})
}

func (f *Field[T]) appendPred2(op Op, v T) *Field[T] {
	return f.appendPred(Predicate[T]{
		Op: op,
		V:  v,
	})
}

func (f *Field[T]) appendType(t TokenType) *Field[T] {
	return f.append(Token[T]{Type: t})
}

// EQ is `=` in SQL.
func (f *Field[T]) EQ(v T) *Field[T] { return f.appendPred2(opEQ, v) }

// NEQ is `<>` in SQL.
func (f *Field[T]) NEQ(v T) *Field[T] { return f.appendPred2(opNEQ, v) }

// GT is `>` in SQL.
func (f *Field[T]) GT(v T) *Field[T] { return f.appendPred2(opGT, v) }

// GTE is `>=` in SQL.
func (f *Field[T]) GTE(v T) *Field[T] { return f.appendPred2(opGTE, v) }

// LT is `<` in SQL.
func (f *Field[T]) LT(v T) *Field[T] { return f.appendPred2(opLT, v) }

// LTE is `<=` in SQL.
func (f *Field[T]) LTE(v T) *Field[T] { return f.appendPred2(opLTE, v) }

// In is `IN` in SQL.
func (f *Field[T]) In(vs []T) *Field[T] {
	return f.appendPred(Predicate[T]{
		Op: opIn,
		Vs: vs,
	})
}

// NotIn is `NOT IN` in SQL.
func (f *Field[T]) NotIn(vs []T) *Field[T] {
	return f.appendPred(Predicate[T]{
		Op: opNotIn,
		Vs: vs,
	})
}

// Like is `LIKE` in SQL.
func (f *Field[T]) Like(v T) *Field[T] {
	return f.appendPred(Predicate[T]{
		Op: opLike,
		V:  v,
	})
}

// IsNull is `IS NULL` in SQL.
func (f *Field[T]) IsNull() *Field[T] {
	return f.appendPred(Predicate[T]{
		Op: opIsNull,
	})
}

// IsNull is `IS NOT NULL` in SQL.
func (f *Field[T]) IsNotNull() *Field[T] {
	return f.appendPred(Predicate[T]{
		Op: opNotNull,
	})
}

// Not add the `NOT` operator to the next predicate.
func (f *Field[T]) Not() *Field[T] { return f.appendType(Not) }

// Or add the `OR` operator to the next predicate (default `AND`).
func (f *Field[T]) Or() *Field[T] { return f.appendType(Or) }

// Zero makes the next predicate not skip zero value.
//
// Attention: it has no effect on [Group].
func (f *Field[T]) Zero() *Field[T] { return f.appendType(Zero) }

// Group groups the sub FP.
func (f *Field[T]) Group(fn func(f *Field[T])) *Field[T] {
	f2 := NewField[T]()
	fn(f2)
	return f.append(Token[T]{
		Type:  Group,
		Group: f2,
	})
}

func (f *Field[T]) BuildFieldBun(name string, qb bun.QueryBuilder) {
	not, or, zero := false, false, false
	for _, pg := range f.Tokens {
		t, p, group := pg.Type, pg.Pred, pg.Group
		switch t {
		case Or:
			or = true
		case Not:
			not = true
		case Zero:
			zero = true
		case Pred:
			if p.IsNop() && !zero {
				continue
			}

			zero = false
			var qs string
			args := []any{bun.Ident(name)}
			if not {
				qs += "NOT "
				not = false
			}

			qs += "? " + p.Op.String()
			if !p.Op.IsUnary() {
				if p.Op.IsIn() {
					qs += " (?)"
					args = append(args, bun.In(p.Vs))
				} else {
					qs += " ?"
					args = append(args, p.V)
				}
			}
			switch {
			case or:
				qb.WhereOr(qs, args...)
				or = false
			default:
				qb.Where(qs, args...)
			}
		case Group:
			var op string
			if or {
				op = " OR "
				or = false
			} else {
				op = " AND "
			}
			if not {
				op += "NOT "
				not = false
			}
			qb.WhereGroup(op, func(qb2 bun.QueryBuilder) bun.QueryBuilder {
				group.BuildFieldBun(name, qb2)
				return qb2
			})
		}
	}
}
