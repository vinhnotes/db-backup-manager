package manager

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/url"

	"github.com/lib/pq"
)

// openPostgres opens a lib/pq pool for a postgresql:// URL. lib/pq rejects
// libpq's "prefer" and "allow" sslmodes, so they are emulated here:
// prefer tries SSL first and falls back to plain if the server has no SSL;
// allow tries plain first and retries with SSL if that fails.
func openPostgres(dsn string) (*sql.DB, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	mode := q.Get("sslmode")
	if mode != "prefer" && mode != "allow" {
		c, err := pq.NewConnector(dsn)
		if err != nil {
			return nil, err
		}
		return sql.OpenDB(c), nil
	}
	withMode := func(m string) (*pq.Connector, error) {
		q.Set("sslmode", m)
		v := *u
		v.RawQuery = q.Encode()
		return pq.NewConnector(v.String())
	}
	first, second := "require", "disable"
	if mode == "allow" {
		first, second = "disable", "require"
	}
	primary, err := withMode(first)
	if err != nil {
		return nil, err
	}
	fallback, err := withMode(second)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(&fallbackConnector{primary: primary, fallback: fallback, preferSSL: mode == "prefer"}), nil
}

type fallbackConnector struct {
	primary, fallback *pq.Connector
	preferSSL         bool
}

func (c *fallbackConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.primary.Connect(ctx)
	if err == nil || ctx.Err() != nil {
		return conn, err
	}
	// prefer only drops SSL when the server says it does not support it.
	if c.preferSSL && !errors.Is(err, pq.ErrSSLNotSupported) {
		return nil, err
	}
	return c.fallback.Connect(ctx)
}

func (c *fallbackConnector) Driver() driver.Driver { return c.primary.Driver() }
