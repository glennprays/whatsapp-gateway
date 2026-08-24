package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/google/uuid"
)

// postgresKeepaliveParams are appended to a Postgres DSN (when not already set)
// so a silently-dropped pooled connection is detected in ~1 minute instead of
// never. Without them, a NAT/firewall black-hole between the gateway and the DB
// leaves any goroutine that picks the dead connection blocked forever inside
// lib/pq's socket read — observed in production as whatsmeow sends and inbound
// message decryption hanging indefinitely while other connections worked fine.
// Detection math: idle 30s, then probes every 10s, give up after 5 failures.
var postgresKeepaliveParams = []struct{ key, val string }{
	{"connect_timeout", "10"},
	{"keepalives", "1"},
	{"keepalives_idle", "30"},
	{"keepalives_interval", "10"},
	{"keepalives_count", "5"},
}

// normalizePostgresDSN injects the keepalive/connect-timeout parameters into a
// Postgres DSN. Both supported forms are handled: URL form
// (postgresql://user:pass@host/db?sslmode=...) and lib/pq keyword form
// ("host=... user=... ..."). Parameters already present in the DSN win.
func normalizePostgresDSN(dsn string) string {
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return dsn // unparseable: let the driver surface the error as-is
		}
		q := u.Query()
		for _, p := range postgresKeepaliveParams {
			if q.Get(p.key) == "" {
				q.Set(p.key, p.val)
			}
		}
		u.RawQuery = q.Encode()
		return u.String()
	}

	// Keyword/value form: only append what's missing.
	have := map[string]bool{}
	for _, kv := range strings.Fields(dsn) {
		if k, _, ok := strings.Cut(kv, "="); ok {
			have[strings.ToLower(k)] = true
		}
	}
	var sb strings.Builder
	sb.WriteString(dsn)
	for _, p := range postgresKeepaliveParams {
		if !have[p.key] {
			sb.WriteString(" " + p.key + "=" + p.val)
		}
	}
	return sb.String()
}

func NewConnection(logger *log.Logger, cfg *config.Config, driverName string, dataSouceName string) (*sql.DB, error) {
	dbTraceID := fmt.Sprintf("DB-INIT:%s", uuid.New().String())

	if driverName == "sqlite" {
		dataSouceName = normalizeSQLiteDSN(dataSouceName)
	} else {
		dataSouceName = normalizePostgresDSN(dataSouceName)
	}

	db, err := sql.Open(driverName, dataSouceName)
	if err != nil {
		logger.Warn(dbTraceID, "Error Open Database", log.Error(err))
		return nil, err
	}

	if driverName == "sqlite" {
		// SQLite allows only one writer even in WAL mode; a single pooled
		// connection makes database/sql queue all callers instead of letting
		// competing connections fail with "database is locked".
		maxOpen, maxIdle := sqlitePoolSettings()
		db.SetMaxOpenConns(maxOpen)
		db.SetMaxIdleConns(maxIdle)
		db.SetConnMaxLifetime(0)
		logger.Info(dbTraceID, fmt.Sprintf(
			"SQLite datastore: WAL/busy_timeout pragmas applied, pool clamped to %d connection(s)", maxOpen,
		), nil)
	} else {
		db.SetMaxOpenConns(cfg.DBMaxOpenConns)
		db.SetMaxIdleConns(cfg.DBMaxIdleConns)
		db.SetConnMaxLifetime(time.Duration(cfg.DBConnMaxLifeMins) * time.Minute)
	}

	if err = db.Ping(); err != nil {
		logger.Warn(dbTraceID, "Error Ping Database", log.Error(err))
		return nil, err
	}
	logger.Info(dbTraceID, "Database connection established successfully", nil)
	return db, nil
}
