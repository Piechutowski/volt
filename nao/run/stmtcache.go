// Package run performs the effects of nao/rt over database/sql (spec
// §values-and-effects): it holds the connection pool, the statement
// cache and the logger, the mutable state of the data layer, and is
// imported by main and the generated route files alone.
package run

import (
	"container/list"
	"context"
	"database/sql"
	"errors"
	"sync"
)

// StmtCache is a prepared-statement cache keyed by rendered SQL text.
// Eviction is LRU; a statement evicted or superseded is closed once no
// caller that Prepare handed it to has released it.
//
// A cache is bound to the handle its statements were prepared on: use one
// StmtCache per *sql.DB and do not feed it transaction handles.
type StmtCache struct {
	mu      sync.Mutex
	max     int
	order   *list.List
	entries map[string]*list.Element
	closed  bool
}

type cacheEntry struct {
	sql   string
	stmt  *sql.Stmt
	users int
	gone  bool
}

// NewStmtCache returns a cache holding at most max prepared statements.
// max <= 0 means an unbounded cache.
func NewStmtCache(max int) *StmtCache {
	return &StmtCache{
		max:     max,
		order:   list.New(),
		entries: map[string]*list.Element{},
	}
}

// Prepare returns the cached statement for query, preparing and caching
// it on first use, and the function that releases it. The statement is
// shared: the caller does not Close it, and calls release once its
// statement has run, so that an eviction meanwhile closes it only then.
func (c *StmtCache) Prepare(ctx context.Context, db *sql.DB, query string) (stmt *sql.Stmt, release func(), err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, nil, errors.New("run.StmtCache: cache is closed")
	}
	if el, ok := c.entries[query]; ok {
		c.order.MoveToFront(el)
		stmt, release, err := c.heldLocked(el.Value.(*cacheEntry))
		c.mu.Unlock()
		return stmt, release, err
	}
	c.mu.Unlock()

	prepared, err := db.PrepareContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		_ = prepared.Close()
		return nil, nil, errors.New("run.StmtCache: cache is closed")
	}
	if el, ok := c.entries[query]; ok {
		_ = prepared.Close()
		c.order.MoveToFront(el)
		return c.heldLocked(el.Value.(*cacheEntry))
	}
	e := &cacheEntry{sql: query, stmt: prepared}
	c.entries[query] = c.order.PushFront(e)
	if c.max > 0 && c.order.Len() > c.max {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		old := oldest.Value.(*cacheEntry)
		delete(c.entries, old.sql)
		c.retireLocked(old)
	}
	return c.heldLocked(e)
}

func (c *StmtCache) heldLocked(e *cacheEntry) (*sql.Stmt, func(), error) {
	e.users++
	var once sync.Once
	return e.stmt, func() { once.Do(func() { c.releaseOne(e) }) }, nil
}

func (c *StmtCache) releaseOne(e *cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e.users--
	if e.gone && e.users == 0 {
		_ = e.stmt.Close()
	}
}

func (c *StmtCache) retireLocked(e *cacheEntry) error {
	e.gone = true
	if e.users > 0 {
		return nil
	}
	return e.stmt.Close()
}

// Len reports the number of cached statements.
func (c *StmtCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Close closes every cached statement and rejects further use; one a
// caller still holds closes when it is released.
func (c *StmtCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	var errs []error
	for el := c.order.Front(); el != nil; el = el.Next() {
		if err := c.retireLocked(el.Value.(*cacheEntry)); err != nil {
			errs = append(errs, err)
		}
	}
	c.order.Init()
	c.entries = map[string]*list.Element{}
	return errors.Join(errs...)
}
