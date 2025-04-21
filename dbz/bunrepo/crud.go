package bunrepo

import (
	"fmt"
	"reflect"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"

	"github.com/adobaai/pkg/dbz"
	"github.com/adobaai/pkg/strz"
)

// For sets row locking for Bun ORM.
func For(q *bun.SelectQuery, f dbz.For) *bun.SelectQuery {
	if f == dbz.Nothing {
		return q
	}

	s := ""
	name := q.DB().Dialect().Name()
	switch name {
	case dialect.PG:
		switch f {
		case dbz.Update:
			s = "UPDATE"
		case dbz.Share:
			s = "SHARE"
		}
	}
	if s == "" {
		panic(fmt.Sprintf("bunrepo: unsupported row locking mode %v for %s", f, name))
	}
	return q.For(s)
}

func List(q *bun.SelectQuery, p dbz.ListParams) *bun.SelectQuery {
	if p == nil {
		return q
	}

	p = dbz.SafeList(p)
	return q.Limit(int(p.GetLimit())).
		Offset(int(p.GetOffset())).
		Order(p.GetOrders()...)
}

// BuildQuery builds a [bun.QueryBuilder] func from a query struct,
// which can be passed to the `ApplyQueryBuilder' func.
//
// It will ignore zero value fields and use the `fp` tag as the column name.
// If no tag, it will generate column names from struct field names
// by underscoring them.
// For example, struct field `UserID` gets column name `user_id`.
// If the `fp` tag is set to "-", the field will be skipped.
func BuildQuery(p any) (func(bun.QueryBuilder) bun.QueryBuilder, error) {
	if p == nil {
		return func(qb bun.QueryBuilder) bun.QueryBuilder { return qb }, nil
	}

	v := reflect.ValueOf(p)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil,
			fmt.Errorf("the p should be a struct or struct pointer, got %v", v.Kind())
	}

	return func(qb bun.QueryBuilder) (res bun.QueryBuilder) {
		t := v.Type()
		for i := range v.NumField() {
			f := v.Field(i)
			fi := f.Interface()
			if f.IsZero() {
				continue
			}

			if _, ok := fi.(dbz.For); ok {
				continue
			}
			if _, ok := fi.(dbz.ListParams); ok {
				continue
			}

			ft := t.Field(i)
			fName := ft.Tag.Get("fp")
			if fName == "-" {
				continue
			}
			if fName == "" {
				fName = strz.Underscore(ft.Name)
			}

			b, ok := fi.(interface {
				BuildFieldBun(name string, qb bun.QueryBuilder)
			})
			if ok {
				b.BuildFieldBun(fName, qb)
			} else {
				qb.Where(fName+" = ?", fi)
			}
		}
		return qb
	}, nil
}
