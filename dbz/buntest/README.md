# buntest

Local sqlmock-backed Bun connections and generated SQL assertions.
Import `github.com/adobaai/pkg/dbz/buntest` from the **dbz module**.

Provides direct sqlmock expectations, automatic cleanup, synchronized SQL capture,
configurable MySQL version discovery, and simple UPDATE assignment checks.
See the Go API comments for usage and limitations.

```go
func TestUpdateSQL(t *testing.T) {
    db, mock, sql := buntest.New(t, pgdialect.New())
    mock.ExpectExec(`^UPDATE `).WillReturnResult(sqlmock.NewResult(0, 1))

    _, err := db.NewUpdate().Table("items").
        Set("count = count + ?", 1).Where("id = ?", 12).Exec(t.Context())
    require.NoError(t, err)
    buntest.Assignments(t, sql.Last(t), "count")
}
```
