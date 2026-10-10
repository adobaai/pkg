package buntest

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/dialect/feature"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/schema"
)

func TestRecording(t *testing.T) {
	db, mock, recorder := New(t, pgdialect.New())
	mock.ExpectExec(`^SELECT [12]$`).WillReturnResult(sqlmock.NewResult(0, 1))
	_, err := db.ExecContext(t.Context(), "SELECT 1")
	require.NoError(t, err)
	require.Equal(t, "SELECT 1", recorder.Last(t))

	mock.ExpectExec(`^SELECT 2$`).WillReturnResult(sqlmock.NewResult(0, 1))
	_, err = db.ExecContext(t.Context(), "SELECT 3")
	require.Error(t, err)
	require.Equal(t, "SELECT 1", recorder.Last(t), "regex mismatches are not recorded")
	_, err = db.ExecContext(t.Context(), "SELECT 2")
	require.NoError(t, err)
	require.Equal(t, "SELECT 2", recorder.Last(t))

	wantErr := errors.New("mock execution failure")
	mock.ExpectExec(`^SELECT 4$`).WillReturnError(wantErr)
	_, err = db.ExecContext(t.Context(), "SELECT 4")
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, "SELECT 4", recorder.Last(t), "matched errors are still recorded")
}

func TestTransactionsAndResults(t *testing.T) {
	for _, commit := range []bool{true, false} {
		t.Run(fmt.Sprintf("commit=%v", commit), func(t *testing.T) {
			db, mock, recorder := New(t, pgdialect.New())
			mock.ExpectBegin()
			mock.ExpectExec(`^UPDATE items SET count = count \+ 1$`).
				WillReturnResult(sqlmock.NewResult(7, 2))
			mock.ExpectQuery(`^SELECT 42$`).RowsWillBeClosed().
				WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(42))
			if commit {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			tx, err := db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			defer func() {
				if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
					t.Error(err)
				}
			}()
			query := "UPDATE items SET count = count + 1"
			result, err := tx.ExecContext(t.Context(), query)
			require.NoError(t, err)
			id, err := result.LastInsertId()
			require.NoError(t, err)
			require.EqualValues(t, 7, id)
			count, err := result.RowsAffected()
			require.NoError(t, err)
			require.EqualValues(t, 2, count)
			var value int
			err = tx.NewRaw("SELECT 42").Scan(t.Context(), &value)
			require.NoError(t, err)
			require.Equal(t, 42, value)
			require.Equal(t, "SELECT 42", recorder.Last(t))
			if commit {
				require.NoError(t, tx.Commit())
			} else {
				require.NoError(t, tx.Rollback())
			}
		})
	}
}

func TestMySQLInitialization(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options []Option
		cte     bool
		alias   bool
	}{
		{name: "default 8.0.36", cte: true, alias: true},
		{name: "5.7", options: []Option{WithMySQLVersion("5.7.44")}},
		{name: "8.0.15", options: []Option{WithMySQLVersion("8.0.15")}, cte: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := mysqldialect.New()
			db, mock, recorder := New(t, d, tc.options...)
			require.NoError(t, mock.ExpectationsWereMet(), "version discovery consumed")
			require.Equal(t, tc.cte, d.Features().Has(feature.CTE))
			require.Equal(t, tc.alias, d.Features().Has(feature.DeleteTableAlias))
			require.Equal(t, "no SQL matched", fatalMessage(t, func(tb testing.TB) {
				recorder.Last(tb)
			}))
			mock.ExpectExec(`^SELECT 1$`).WillReturnResult(sqlmock.NewResult(0, 1))
			_, err := db.ExecContext(t.Context(), "SELECT 1")
			require.NoError(t, err)
			require.Equal(t, "SELECT 1", recorder.Last(t))
		})
	}
	t.Run("version option ignored by PostgreSQL", func(t *testing.T) {
		_, mock, _ := New(t, pgdialect.New(), WithMySQLVersion("5.7.44"))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// Embed TB to retain its private method while intercepting expected assertion failures.
type assertionTB struct{ testing.TB }

func (t *assertionTB) Fatal(args ...any) { panic(fmt.Sprint(args...)) }
func (t *assertionTB) Fatalf(format string, args ...any) {
	panic(fmt.Sprintf(format, args...))
}

func fatalMessage(t *testing.T, fn func(testing.TB)) (message string) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			message = recovered.(string)
		}
	}()
	fn(&assertionTB{TB: t})
	return ""
}

func TestAssignmentsScalarStrings(t *testing.T) {
	cases := []struct {
		name, failure string
		want          []string
		extraColumn   bool
	}{
		{name: "exact target", want: []string{"name"}},
		{
			name: "extra target fails", want: []string{"name", "count"},
			failure: "assignment targets [name]",
		},
		{
			name: "real target after literal", want: []string{"name", "count2"},
			extraColumn: true,
		},
	}
	for _, mysql := range []bool{false, true} {
		for _, value := range []string{
			"literal, count = 1",
			"literal SET count = 1 FROM items WHERE id = 1 RETURNING count",
			"it's quoted, count = 1",
			"literal\\",
			"literal\\', count = 1",
		} {
			for _, tc := range cases {
				name := fmt.Sprintf("mysql=%v/%s/%s", mysql, value, tc.name)
				t.Run(name, func(t *testing.T) {
					var d schema.Dialect = pgdialect.New()
					if mysql {
						d = mysqldialect.New()
					}
					db, _, _ := New(t, d)
					update := db.NewUpdate().Table("items").
						Set("name = ?", value).Where("id = ?", 1)
					if tc.extraColumn {
						update.Set("count2 = ?", 2)
					}
					query := update.String()
					message := fatalMessage(t, func(tb testing.TB) {
						Assignments(tb, query, tc.want...)
					})
					if tc.failure == "" {
						require.Empty(t, message, "SQL: %s", query)
					} else {
						require.Contains(t, message, tc.failure)
					}
				})
			}
		}
	}
}

func TestAssignments(t *testing.T) {
	for _, tc := range []struct {
		name, query, failure string
		want                 []string
	}{
		{
			name: "quoted and unquoted scalar targets",
			query: "UPDATE items SET \"name\" = 'new', count = count + 1, " +
				"`deleted_at` = NULL WHERE id = 1 RETURNING *",
			want: []string{"deleted_at", "name", "count"},
		},
		{
			name: "CTE and FROM",
			query: "WITH vals AS (SELECT 1) UPDATE items SET count = vals.count " +
				"FROM vals WHERE items.id = vals.id RETURNING count",
			want: []string{"count"},
		},
		{
			name: "duplicate", query: "UPDATE items SET count = 1, count = 2",
			want: []string{"count"}, failure: "assignment targets [count count]",
		},
		{
			name: "extra", query: "UPDATE items SET count = 1, name = 'new'",
			want: []string{"count"}, failure: "assignment targets [count name]",
		},
		{
			name: "missing", query: "UPDATE items SET count = 1",
			want: []string{"count", "name"}, failure: "want [count name]",
		},
		{
			name:  "digit in column name",
			query: "UPDATE items SET count = 1, count2 = 2",
			want:  []string{"count", "count2"},
		},
		{
			name: "extra digit column", query: "UPDATE items SET count = 1, count2 = 2",
			want: []string{"count"}, failure: "assignment targets [count count2]",
		},
		{
			name:  "quoted digit columns",
			query: "UPDATE items SET \"count1\" = 1, `count2` = 2",
			want:  []string{"count1", "count2"},
		},
		{
			name:  "parenthesized scalar expression",
			query: "UPDATE items SET count = COALESCE(other, 0), name = 'new'",
			want:  []string{"count", "name"},
		},
		{
			name:  "unsupported target fails closed",
			query: "UPDATE items SET count = 1, items.name = 'new'",
			want:  []string{"count"}, failure: "unsupported assignment",
		},
		{
			name: "trailing comma", query: "UPDATE items SET count = 1, ",
			want: []string{"count"}, failure: "unsupported assignment",
		},
		{
			name: "missing value", query: "UPDATE items SET count = ",
			want: []string{"count"}, failure: "unsupported assignment",
		},
		{
			name: "unterminated string", query: "UPDATE items SET name = 'unterminated",
			want: []string{"name"}, failure: "unterminated quoted SQL",
		},
		{
			name: "unbalanced parentheses", query: "UPDATE items SET count = (1",
			want: []string{"count"}, failure: "unbalanced parentheses",
		},
		{
			name: "SQL comment", query: "UPDATE items SET count = 1 /* ignored */",
			want: []string{"count"}, failure: "unsupported SQL comment",
		},
		{
			name:  "dollar quoted value",
			query: "UPDATE items SET name = $$literal, count = 1$$",
			want:  []string{"name"}, failure: "unsupported dollar expression",
		},
		{name: "no SET", query: "SELECT 1", failure: "missing SET"},
		{name: "empty SET", query: "UPDATE items SET ", failure: "assignment targets []"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := append([]string(nil), tc.want...)
			message := fatalMessage(t, func(tb testing.TB) {
				Assignments(tb, tc.query, tc.want...)
			})
			if tc.failure == "" {
				require.Empty(t, message)
			} else {
				require.Contains(t, message, tc.failure)
			}
			require.Equal(t, original, tc.want, "caller target order must be preserved")
		})
	}
}

type cleanupTB struct {
	testing.TB
	cleanup func()
	errors  []string
}

func (t *cleanupTB) Cleanup(fn func()) { t.cleanup = fn }
func (t *cleanupTB) Error(args ...any) { t.errors = append(t.errors, fmt.Sprint(args...)) }

func TestCleanup(t *testing.T) {
	t.Run("multiple pooled connections", func(t *testing.T) {
		tb := &cleanupTB{TB: t}
		db, mock, _ := New(tb, pgdialect.New())
		t.Cleanup(tb.cleanup)
		mock.ExpectBegin()
		mock.ExpectExec(`^SELECT 1$`).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectRollback()
		tx, err := db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		defer func() {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				t.Error(err)
			}
		}()
		_, err = db.ExecContext(t.Context(), "SELECT 1")
		require.NoError(t, err)
		require.Equal(t, 2, db.Stats().OpenConnections)
		require.NoError(t, tx.Rollback())
		tb.cleanup()
		require.Empty(t, tb.errors)
		require.Zero(t, db.Stats().OpenConnections)
		require.NoError(t, mock.ExpectationsWereMet(), "all close expectations consumed")
	})
	t.Run("unmet expectations fail cleanup", func(t *testing.T) {
		tb := &cleanupTB{TB: t}
		_, mock, _ := New(tb, pgdialect.New())
		mock.ExpectExec(`^SELECT 99$`).WillReturnResult(sqlmock.NewResult(0, 1))
		tb.cleanup()
		require.NotEmpty(t, tb.errors)
		require.Contains(t, strings.Join(tb.errors, "\n"), "remaining expectation")
	})
}

func TestConcurrentRecorder(t *testing.T) {
	db, mock, recorder := New(t, pgdialect.New())
	mock.MatchExpectationsInOrder(false)
	const workers = 16
	for range workers + 1 {
		mock.ExpectExec(`^SELECT 1$`).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	_, err := db.ExecContext(t.Context(), "SELECT 1")
	require.NoError(t, err)
	var group sync.WaitGroup
	for range workers {
		group.Go(func() {
			_, err := db.ExecContext(t.Context(), "SELECT 1")
			if err != nil {
				t.Error(err)
			}
			require.Equal(t, "SELECT 1", recorder.Last(t))
		})
	}
	group.Wait()
}
