package bunrepo

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/extra/bundebug"

	"github.com/adobaai/pkg/dbz"
	"github.com/adobaai/pkg/dbz/predicate"
	"github.com/adobaai/pkg/testingz"
)

func newDB() (res *bun.DB, err error) {
	var dsnMemoryShared = "file::memory:?cache=shared"
	sqldb, err := sql.Open(sqliteshim.ShimName, dsnMemoryShared)
	if err != nil {
		return
	}

	res = bun.NewDB(sqldb, sqlitedialect.New())
	res.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	return
}

type Payment struct {
	bun.BaseModel `bun:",alias:p"`

	ID         int    `bun:",pk"`
	ProviderID string `bun:",unique"`
	Summary    string
	CreatedAt  time.Time `bun:"default:CURRENT_TIMESTAMP"`
}

func TestPayment(t *testing.T) {
	bdb, err := newDB()
	require.NoError(t, err)
	ctx := context.Background()
	testingz.R(bdb.NewCreateTable().Model((*Payment)(nil)).Exec(ctx)).NoError(t)
	repo := New[Payment](bdb)

	p := &Payment{
		ID:         2234,
		ProviderID: "I-2234",
	}
	ps := []*Payment{
		p,
		{ID: 3234, ProviderID: "I-3234"},
		{ID: 4234, ProviderID: "I-4234"},
		{ID: 5234, ProviderID: "I-5234", Summary: "X-5234"},
	}
	testingz.R(repo.Addm(ctx, ps)).NoError(t)
	p5 := Payment{
		ProviderID: ps[3].ProviderID,
		Summary:    "X-5234-2",
	}
	testingz.R(repo.Add(ctx, &p5, On("CONFLICT (provider_id) DO UPDATE"))).NoError(t)

	defer func() {
		t.Run("Del", func(t *testing.T) {
			res, err := repo.Del(ctx, &Payment{Summary: p5.Summary}, WherePK("summary"))
			require.NoError(t, err)
			testingz.R(res.RowsAffected()).NoError(t).Equal(1)

			del := Payment{ID: ps[2].ID}
			res, err = repo.Del(ctx, &del, Returning("*"))
			require.NoError(t, err)
			testingz.R(res.RowsAffected()).NoError(t).Equal(1)
			assert.Equal(t, ps[2].ProviderID, del.ProviderID)

			ret := Payment{}
			res = testingz.R(
				repo.Delf(ctx, &ret, func(q *bun.DeleteQuery) *bun.DeleteQuery {
					return q.Where("provider_id = ?", ps[1].ProviderID)
				})).
				NoError(t).
				V()
			testingz.R(res.RowsAffected()).NoError(t).Equal(1)
			// TODO assert.Equal(t, ps[1].ID, ret.ID)

			res = testingz.R(repo.Delm(ctx, ps)).NoError(t).V()
			testingz.R(res.RowsAffected()).NoError(t).Equal(1)
			assert.ErrorIs(t, repo.Get(ctx, &Payment{ID: p.ID}), sql.ErrNoRows)
		})
	}()

	t.Run("Upd", func(t *testing.T) {
		testingz.
			R(repo.Updf(ctx, &Payment{
				ProviderID: p.ProviderID,
				Summary:    "world",
			}, func(q *bun.UpdateQuery) *bun.UpdateQuery {
				return q.WherePK("provider_id").Column("summary")
			})).
			NoError(t)

		got := &Payment{
			ID:         p.ID,
			ProviderID: "J-2243",
		}
		testingz.R(repo.Upd(ctx, got, Columns("provider_id"), Returning("summary"))).NoError(t)
		assert.Equal(t, "world", got.Summary)

		got = &Payment{ID: p.ID}
		testingz.R(got, repo.Get(ctx, got)).NoError(t).Do(func(t *testing.T, it *Payment) {
			assert.Equal(t, "world", it.Summary)
			assert.Equal(t, "J-2243", it.ProviderID)
			assert.WithinDuration(t, time.Now(), it.CreatedAt, time.Second)
		})

		testingz.
			R(repo.Upd(ctx, &Payment{
				ID:         p.ID,
				ProviderID: "J-2243",
			}, IncludeZero())).
			NoError(t)

		got = &Payment{ID: p.ID}
		testingz.R(got, repo.Get(ctx, got)).NoError(t).Do(func(t *testing.T, it *Payment) {
			assert.Equal(t, "", it.Summary)
			assert.Equal(t, "J-2243", it.ProviderID)
		})
	})

	t.Run("WherePK", func(t *testing.T) {
		u := &Payment{
			ProviderID: "J-2243",
			Summary:    "hi there",
		}
		testingz.R(repo.Upd(ctx, u, WherePK("provider_id"))).NoError(t)

		got := &Payment{ID: p.ID}
		testingz.R(got, repo.Get(ctx, got)).NoError(t).
			Do(func(t *testing.T, it *Payment) {
				assert.Equal(t, "hi there", it.Summary)
			})
	})

	t.Run("Where", func(t *testing.T) {
		testingz.R(repo.Upd(ctx, &Payment{Summary: "good"}, Where("provider_id = ?", "J-2243"))).
			NoError(t)

		got := &Payment{ID: p.ID}
		testingz.R(got, repo.Get(ctx, got)).NoError(t).
			Do(func(t *testing.T, it *Payment) {
				assert.Equal(t, "good", it.Summary)
			})

	})

	t.Run("Columns", func(t *testing.T) {
		got := &Payment{ID: p.ID}
		testingz.R(got, repo.Get(ctx, got, Columns("created_at"))).NoError(t).
			Do(func(t *testing.T, it *Payment) {
				assert.Equal(t, "", it.Summary)
			})
	})

	type ListPaymentsParams struct {
		ID *predicate.Field[int]
	}

	t.Run("Getm", func(t *testing.T) {
		_, n, err := repo.Getm(ctx, &dbz.BaseList{
			Offset: 1,
		}, ListPaymentsParams{
			ID: predicate.GT(3000),
		}, Count(), Where("id != ?", 5234))
		require.NoError(t, err)
		assert.Equal(t, 2, n)

		got, n, err := repo.Getm(ctx, &dbz.BaseList{
			Limit:    3,
			Offset:   1,
			Orders:   []string{"summary"},
			MaxLimit: 2,
		}, ListPaymentsParams{
			ID: predicate.GT(3000),
		}, Where("id != ?", 5234))
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, 0, n)
		assert.Equal(t, 4234, got[0].ID)
		assert.Equal(t, "I-4234", got[0].ProviderID)
	})
}
