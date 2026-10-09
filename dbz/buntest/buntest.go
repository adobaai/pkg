// Package buntest provides sqlmock-backed Bun connections and generated SQL assertions.
package buntest

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/uptrace/bun/schema"
)

// Recorder retains SQL accepted by the regex matcher, excluding dialect initialization.
// Its methods are safe for concurrent use. Last observes matcher order, not completion order.
// Do not copy a Recorder after first use. Unordered sqlmock matching can record a query twice
// or record a candidate whose arguments subsequently fail; this is not an execution log.
type Recorder struct {
	mu      sync.RWMutex
	queries []string
}

func (r *Recorder) record(query string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queries = append(r.queries, query)
}

// Last returns the latest matched SQL, failing the test if no query has matched.
// Synchronize query-producing goroutines before asserting a particular last query.
func (r *Recorder) Last(t testing.TB) string {
	t.Helper()
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.queries) == 0 {
		t.Fatal("no SQL matched")
	}
	return r.queries[len(r.queries)-1]
}

type config struct{ mysqlVersion string }

// Option configures a mock connection.
type Option func(*config)

// WithMySQLVersion sets the response to MySQL's SELECT version() discovery query.
// The default is 8.0.36. This option has no effect on other dialects.
func WithMySQLVersion(version string) Option {
	return func(c *config) { c.mysqlVersion = version }
}

// New creates a Bun DB with regex SQL matching and directly accessible sqlmock expectations.
// Cleanup verifies expectations and closes every pooled connection. Finish transactions and
// query-producing goroutines before cleanup; callers should leave closing the DB to this helper.
func New(t testing.TB, d schema.Dialect, opts ...Option) (*bun.DB, sqlmock.Sqlmock, *Recorder) {
	t.Helper()
	cfg := config{mysqlVersion: "8.0.36"}
	for _, opt := range opts {
		opt(&cfg)
	}
	recorder := new(Recorder)
	matcher := sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		if err := sqlmock.QueryMatcherRegexp.Match(expected, actual); err != nil {
			return err
		}
		recorder.record(actual)
		return nil
	})
	sqldb, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	if err != nil {
		t.Fatal(err)
	}
	if d.Name() == dialect.MySQL {
		rows := sqlmock.NewRows([]string{"version"}).AddRow(cfg.mysqlVersion)
		mock.ExpectQuery(`^SELECT version\(\)$`).
			WillReturnRows(rows)
	}
	db := bun.NewDB(sqldb, d)
	recorder.mu.Lock()
	recorder.queries = nil
	recorder.mu.Unlock()
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		// Another connection may be acquired while a transaction holds the first.
		for range sqldb.Stats().OpenConnections {
			mock.ExpectClose()
		}
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db, mock, recorder
}

var assignmentTargetPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// Assignments checks the entire UPDATE SET target list, including duplicate targets.
// It supports simple Bun UPDATE shapes with lowercase unqualified column names (including
// digits), optional double quotes or backticks, and scalar assignments. Quoted values use
// Bun's doubled-quote escaping; their contents do not count as targets or clause boundaries.
// It is not a general SQL parser: SQL comments, dollar quoting, and nested UPDATEs are unsupported.
func Assignments(t testing.TB, query string, want ...string) {
	t.Helper()
	got, err := assignmentTargets(query)
	if err != nil {
		t.Fatalf("%s; SQL: %s", err, query)
	}
	slices.Sort(got)
	want = slices.Clone(want)
	slices.Sort(want)
	if len(got) == 0 || !slices.Equal(got, want) {
		t.Fatalf("assignment targets %v, want %v; SQL: %s", got, want, query)
	}
}

func assignmentTargets(query string) ([]string, error) {
	assignments, err := updateAssignments(query)
	if err != nil {
		return nil, err
	}
	got := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		target, value, ok := strings.Cut(assignment, "=")
		target = strings.TrimSpace(target)
		if len(target) >= 2 && (target[0] == '"' || target[0] == '`') &&
			target[len(target)-1] == target[0] {
			target = target[1 : len(target)-1]
		}
		if !ok || strings.TrimSpace(value) == "" ||
			!assignmentTargetPattern.MatchString(target) {
			return nil, fmt.Errorf("unsupported assignment: %s", assignment)
		}
		got = append(got, target)
	}
	return got, nil
}

// updateAssignments splits only at unquoted, top-level SET-list boundaries.
func updateAssignments(query string) ([]string, error) {
	var assignments []string
	start, depth := -1, 0
	for i := 0; i < len(query); i++ {
		switch query[i] {
		case '\'', '"', '`':
			end, err := quotedEnd(query, i)
			if err != nil {
				return nil, err
			}
			i = end
			continue
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced parentheses")
			}
		case '$':
			return nil, fmt.Errorf("unsupported dollar expression")
		}
		if strings.HasPrefix(query[i:], "--") || strings.HasPrefix(query[i:], "/*") {
			return nil, fmt.Errorf("unsupported SQL comment")
		}
		if depth != 0 {
			continue
		}
		switch {
		case start < 0 && strings.HasPrefix(query[i:], " SET "):
			start = i + 5
			i += 4
		case start < 0:
			continue
		case isSetEnd(query[i:]):
			return append(assignments, query[start:i]), nil
		case query[i] == ',':
			assignments = append(assignments, query[start:i])
			start = i + 1
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("missing SET")
	}
	if depth != 0 {
		return nil, fmt.Errorf("unbalanced parentheses")
	}
	if len(assignments) == 0 && strings.TrimSpace(query[start:]) == "" {
		return nil, nil
	}
	return append(assignments, query[start:]), nil
}

func isSetEnd(sql string) bool {
	return strings.HasPrefix(sql, " FROM ") || strings.HasPrefix(sql, " WHERE ") ||
		strings.HasPrefix(sql, " RETURNING ")
}

func quotedEnd(query string, start int) (int, error) {
	quote := query[start]
	for i := start + 1; i < len(query); i++ {
		if query[i] != quote {
			continue
		}
		if i+1 < len(query) && query[i+1] == quote {
			i++
			continue
		}
		return i, nil
	}
	return 0, fmt.Errorf("unterminated quoted SQL")
}
