package database

import (
	"strings"
)

// sqliteDefaultPragmas are applied to every SQLite DSN unless the user
// already set the pragma (any value) themselves:
//   - journal_mode(WAL): writers no longer block readers; without it every
//     write locks the whole database file.
//   - busy_timeout(5000): wait up to 5s for a lock instead of failing
//     instantly with "database is locked".
//   - synchronous(NORMAL): the documented safe/fast pairing with WAL;
//     shortens the time write locks are held.
//
// journal_mode=WAL persists in the database file; switching an existing
// database is automatic and safe on first connection.
var sqliteDefaultPragmas = []struct {
	name  string
	value string
}{
	{"journal_mode", "WAL"},
	{"busy_timeout", "5000"},
	{"synchronous", "NORMAL"},
}

// normalizeSQLiteDSN appends the default pragmas (modernc.org/sqlite
// `_pragma=name(value)` syntax) to a SQLite DSN unless already present.
// It is idempotent, preserves user-supplied parameters and their order,
// and never overrides a pragma the user set to a different value.
//
// String handling is manual on purpose: net/url percent-encodes the
// parentheses in pragma values, which corrupts the DSN.
func normalizeSQLiteDSN(uri string) string {
	base := uri
	query := ""
	if i := strings.IndexByte(uri, '?'); i >= 0 {
		base = uri[:i]
		query = uri[i+1:]
	}

	// Attach the file: scheme to bare paths so query parameters are valid.
	if !strings.HasPrefix(base, "file:") && base != ":memory:" {
		base = "file:" + base
	}

	existing := existingPragmaNames(query)

	params := []string{}
	if query != "" {
		params = append(params, query)
	}
	for _, p := range sqliteDefaultPragmas {
		if _, ok := existing[p.name]; ok {
			continue
		}
		params = append(params, "_pragma="+p.name+"("+p.value+")")
	}

	if len(params) == 0 {
		return base
	}
	return base + "?" + strings.Join(params, "&")
}

// existingPragmaNames extracts the lowercased names of `_pragma=name(value)`
// parameters already present in a DSN query string.
func existingPragmaNames(query string) map[string]struct{} {
	names := make(map[string]struct{})
	for _, param := range strings.Split(query, "&") {
		value, ok := strings.CutPrefix(param, "_pragma=")
		if !ok {
			continue
		}
		name := value
		if i := strings.IndexByte(value, '('); i >= 0 {
			name = value[:i]
		}
		names[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}
	return names
}

// sqlitePoolSettings returns the connection pool limits used for SQLite.
// SQLite allows only one writer even in WAL mode; with a single pooled
// connection database/sql queues all callers onto one serialized
// connection, so a competing connection can never observe a lock.
func sqlitePoolSettings() (maxOpen, maxIdle int) {
	return 1, 1
}
