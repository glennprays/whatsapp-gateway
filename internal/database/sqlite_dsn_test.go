package database

import (
	"testing"
)

func TestNormalizeSQLiteDSN(t *testing.T) {
	const defaults = "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "bare relative path",
			in:   "dbs/whatsapp.db",
			want: "file:dbs/whatsapp.db?" + defaults,
		},
		{
			name: "bare absolute path",
			in:   "/var/data/whatsapp.db",
			want: "file:/var/data/whatsapp.db?" + defaults,
		},
		{
			name: "current production default keeps foreign_keys",
			in:   "file:dbs/whatsapp.db?_pragma=foreign_keys(1)",
			want: "file:dbs/whatsapp.db?_pragma=foreign_keys(1)&" + defaults,
		},
		{
			name: "file scheme without query",
			in:   "file:dbs/whatsapp.db",
			want: "file:dbs/whatsapp.db?" + defaults,
		},
		{
			name: "user journal_mode is never overridden",
			in:   "file:x.db?_pragma=journal_mode(MEMORY)",
			want: "file:x.db?_pragma=journal_mode(MEMORY)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)",
		},
		{
			name: "user busy_timeout is kept, not duplicated",
			in:   "file:x.db?_pragma=busy_timeout(10000)",
			want: "file:x.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)",
		},
		{
			name: "case-insensitive pragma detection",
			in:   "file:x.db?_pragma=JOURNAL_MODE(wal)",
			want: "file:x.db?_pragma=JOURNAL_MODE(wal)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)",
		},
		{
			name: "memory database keeps its form",
			in:   ":memory:",
			want: ":memory:?" + defaults,
		},
		{
			name: "file memory database keeps scheme",
			in:   "file::memory:?cache=shared",
			want: "file::memory:?cache=shared&" + defaults,
		},
		{
			name: "non-pragma params preserved",
			in:   "file:x.db?cache=shared&mode=rwc",
			want: "file:x.db?cache=shared&mode=rwc&" + defaults,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizeSQLiteDSN(c.in)
			if got != c.want {
				t.Errorf("normalizeSQLiteDSN(%q)\n got  %q\n want %q", c.in, got, c.want)
			}
		})
	}
}

func TestNormalizeSQLiteDSN_Idempotent(t *testing.T) {
	inputs := []string{
		"dbs/whatsapp.db",
		"file:dbs/whatsapp.db?_pragma=foreign_keys(1)",
		"file:x.db?_pragma=journal_mode(MEMORY)",
	}
	for _, in := range inputs {
		once := normalizeSQLiteDSN(in)
		twice := normalizeSQLiteDSN(once)
		if once != twice {
			t.Errorf("not idempotent for %q:\n once  %q\n twice %q", in, once, twice)
		}
	}
}

func TestSqlitePoolSettings(t *testing.T) {
	maxOpen, maxIdle := sqlitePoolSettings()
	if maxOpen != 1 || maxIdle != 1 {
		t.Errorf("sqlitePoolSettings() = (%d, %d), want (1, 1)", maxOpen, maxIdle)
	}
}
