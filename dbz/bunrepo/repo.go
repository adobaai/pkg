// Package bunrepo provides a repository pattern implementation for SQL.
package bunrepo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/adobaai/pkg/dbz"
)

// Repo is the repository.
// T is the entity type.
type Repo[T any] struct {
	db        bun.IDB
	returning string
}

func New[T any](db bun.IDB, opts ...RepoOption[T]) *Repo[T] {
	return &Repo[T]{
		db: db,
	}
}

type RepoOption[T any] func(*Repo[T])

func WithReturning[T any](r string) RepoOption[T] {
	return func(repo *Repo[T]) {
		repo.returning = r
	}
}

type GetOption interface {
	ApplyGet(o *getOption)
}

type getOption struct {
	queryOption
	Columns   []string
	ForUpdate bool
	Count     bool
}

// Get returns the entity by id.
func (repo *Repo[T]) Get(ctx context.Context, entity *T, opts ...GetOption) (err error) {
	o := getOption{}
	for _, opt := range opts {
		opt.ApplyGet(&o)
	}
	q := repo.db.NewSelect().Model(entity).Column(o.Columns...).
		ApplyQueryBuilder(o.QueryBuilder(true))

	if o.ForUpdate {
		For(q, dbz.Update)
	}
	err = q.Scan(ctx)
	return
}

// Getm gets multiple entities.
func (repo *Repo[T]) Getm(ctx context.Context, lp dbz.ListParams, p any, opts ...GetOption,
) (res []*T, n int, err error) {
	o := getOption{}
	for _, opt := range opts {
		opt.ApplyGet(&o)
	}

	qb, err := BuildQuery(p)
	if err != nil {
		return nil, 0, fmt.Errorf("build: %w", err)
	}

	q := repo.db.NewSelect().Model(&res).ApplyQueryBuilder(qb).
		ApplyQueryBuilder(o.QueryBuilder(false))
	if o.Count {
		n, err = List(q, lp).ScanAndCount(ctx)
	} else {
		err = List(q, lp).Scan(ctx)
	}
	return
}

type AddOption interface {
	ApplyAdd(*addOption)
}

type addOption struct {
	Columns []string
	On      string
}

// Add adds a new entity to the repository.
func (repo *Repo[T]) Add(ctx context.Context, entity *T, opts ...AddOption,
) (res sql.Result, err error) {
	return repo.add(ctx, entity, opts...)
}

// Addm adds multiple entities to the repository.
func (repo *Repo[T]) Addm(ctx context.Context, entities []*T, opts ...AddOption,
) (res sql.Result, err error) {
	return repo.add(ctx, &entities, opts...)
}

func (repo *Repo[T]) add(ctx context.Context, entities any, opts ...AddOption,
) (res sql.Result, err error) {
	o := addOption{}
	for _, opt := range opts {
		opt.ApplyAdd(&o)
	}
	q := repo.db.NewInsert().Model(entities).Column(o.Columns...)
	if o.On != "" {
		q.On(o.On)
	}
	if repo.returning != "" {
		q.Returning(repo.returning)
	}
	return q.Exec(ctx)
}

type UpdOption interface {
	ApplyUpd(o *updOption)
}

type updOption struct {
	queryOption
	Columns     []string
	IncludeZero bool
}

// Upd updates an entity.
func (repo *Repo[T]) Upd(ctx context.Context, entity *T, opts ...UpdOption,
) (res sql.Result, err error) {
	q := repo.db.NewUpdate().Model(entity)
	return applyUpdOptions(q, opts...).Exec(ctx)
}

// Updf provides more customizations for updation via function.
func (repo *Repo[T]) Updf(
	ctx context.Context,
	entity *T,
	f func(q *bun.UpdateQuery) *bun.UpdateQuery,
) (sql.Result, error) {
	return repo.db.NewUpdate().Model(entity).Apply(f).Exec(ctx)
}

func (repo *Repo[T]) Updm(ctx context.Context, entities []*T) (sql.Result, error) {
	q := repo.db.NewUpdate().Model(&entities).Bulk()
	return q.Exec(ctx)
}

func applyUpdOptions(q *bun.UpdateQuery, opts ...UpdOption) *bun.UpdateQuery {
	o := updOption{}
	for _, opt := range opts {
		opt.ApplyUpd(&o)
	}
	q.Column(o.Columns...).ApplyQueryBuilder(o.QueryBuilder(true))
	if !o.IncludeZero {
		q.OmitZero()
	}
	return q
}

type DelOption interface {
	ApplyDel(o *delOption)
}

type delOption struct {
	queryOption
}

// Del deletes the entity from the repository.
func (repo *Repo[T]) Del(ctx context.Context, entity *T, opts ...DelOption,
) (res sql.Result, err error) {
	o := delOption{}
	for _, opt := range opts {
		opt.ApplyDel(&o)
	}
	return repo.db.NewDelete().Model(entity).
		ApplyQueryBuilder(o.QueryBuilder(true)).
		Exec(ctx)
}

// Delm deletes multiple entities from the repository.
func (repo *Repo[T]) Delm(ctx context.Context, entities []*T, opts ...DelOption,
) (res sql.Result, err error) {
	o := delOption{}
	for _, opt := range opts {
		opt.ApplyDel(&o)
	}
	return repo.db.NewDelete().Model(&entities).
		ApplyQueryBuilder(o.QueryBuilder(true)).
		Exec(ctx)
}

// Delf provides more customizations for deletion via function.
func (repo *Repo[T]) Delf(
	ctx context.Context,
	entity *T,
	f func(q *bun.DeleteQuery) *bun.DeleteQuery,
) (sql.Result, error) {
	return repo.db.NewDelete().Model(entity).Apply(f).Exec(ctx)
}

type QueryOption interface {
	GetOption
	UpdOption
	DelOption
}

type queryOption struct {
	WherePKSet bool
	WherePK    []string
	Wheres     []whereOption
}

func (qo *queryOption) QueryBuilder(defaultPK bool) func(bun.QueryBuilder) bun.QueryBuilder {
	return func(qb bun.QueryBuilder) bun.QueryBuilder {
		for _, wo := range qo.Wheres {
			qb = qb.Where(wo.Query, wo.Args...)
		}
		if qo.WherePKSet || len(qo.Wheres) == 0 && defaultPK {
			qb = qb.WherePK(qo.WherePK...)
		}
		return qb
	}
}

func Where(query string, args ...any) QueryOption {
	return whereOption{
		Query: query,
		Args:  args,
	}
}

type whereOption struct {
	Query string
	Args  []any
}

func (wo whereOption) ApplyGet(o *getOption) {
	o.Wheres = append(o.Wheres, wo)
}

func (wo whereOption) ApplyUpd(o *updOption) {
	o.Wheres = append(o.Wheres, wo)
}

func (wo whereOption) ApplyDel(o *delOption) {
	o.Wheres = append(o.Wheres, wo)
}

// WherePK specifies the columns which will be used in where clause,
// default to primary key.
func WherePK(cols ...string) QueryOption {
	return wherePKOption(cols)
}

type wherePKOption []string

func (wpk wherePKOption) ApplyGet(o *getOption) {
	o.WherePKSet = true
	o.WherePK = wpk
}

func (wpk wherePKOption) ApplyUpd(o *updOption) {
	o.WherePKSet = true
	o.WherePK = wpk
}

func (wpk wherePKOption) ApplyDel(o *delOption) {
	o.WherePKSet = true
	o.WherePK = wpk
}

type AddGetUpdOption interface {
	AddOption
	GetOption
	UpdOption
}

// Columns specifies the columns which will be modified in the update.
func Columns(cols ...string) AddGetUpdOption {
	return columnsOption(cols)
}

type columnsOption []string

func (co columnsOption) ApplyAdd(o *addOption) {
	o.Columns = co
}

func (co columnsOption) ApplyGet(o *getOption) {
	o.Columns = co
}

func (co columnsOption) ApplyUpd(o *updOption) {
	o.Columns = co
}

// ForUpdate locks the rows for an update.
func ForUpdate() GetOption {
	return forUpdateOption(true)
}

type forUpdateOption bool

func (fu forUpdateOption) ApplyGet(o *getOption) {
	o.ForUpdate = bool(fu)
}

// Count count the query when call [Getm].
func Count() GetOption {
	return countOption(true)
}

type countOption bool

func (co countOption) ApplyGet(o *getOption) {
	o.Count = bool(co)
}

type onAdd string

// On performs an upsert operation.
func On(q string) AddOption {
	return onAdd(q)
}

func (oa onAdd) ApplyAdd(o *addOption) {
	o.On = string(oa)
}

// IncludeZero will include zero fields when updating.
func IncludeZero() UpdOption {
	return includeZeroOption(true)
}

type includeZeroOption bool

func (oz includeZeroOption) ApplyUpd(o *updOption) {
	o.IncludeZero = bool(oz)
}
