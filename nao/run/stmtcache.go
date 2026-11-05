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
// Eviction is LRU; a statement evicted or superseded is closed on its
// own goroutine once no caller that Prepare handed it to holds it, since
// closing waits for any step running on a connection it was prepared on.
//
// A cache is bound to the handle its statements were prepared on: use one
// StmtCache per *sql.DB and do not feed it transaction handles.
type StmtCache struct {
	mu      sync.Mutex
	max     int
	order   *list.List
	entries map[string]*list.Element
	closed  bool
	closing int
	drained *sync.Cond
}

type cacheEntry struct {
	sql   string
	stmt  *sql.Stmt
	users int
	gone  bool
}

var errCacheClosed = errors.New("run.StmtCache: cache is closed")

func unheld() {}

// NewStmtCache returns a cache holding at most max prepared statements.
// max <= 0 means an unbounded cache.
func NewStmtCache(max int) *StmtCache {
	c := &StmtCache{
		max:     max,
		order:   list.New(),
		entries: map[string]*list.Element{},
	}
	c.drained = sync.NewCond(&c.mu)
	return c
}

// Prepare returns the cached statement for query, preparing and caching
// it on first use, and the function that releases it. The statement is
// shared: the caller does not Close it, and calls release once it is
// done with it and the rows it returned, so that an eviction meanwhile
// closes it only then; release may be called more than once, and does
// nothing after an error.
func (c *StmtCache) Prepare(ctx context.Context, db *sql.DB, query string) (stmt *sql.Stmt, release func(), err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, unheld, errCacheClosed
	}
	if el, ok := c.entries[query]; ok {
		c.order.MoveToFront(el)
		stmt, release := c.heldLocked(el.Value.(*cacheEntry))
		c.mu.Unlock()
		return stmt, release, nil
	}
	c.mu.Unlock()

	prepared, err := db.PrepareContext(ctx, query)
	if err != nil {
		return nil, unheld, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		c.closeLocked(prepared)
		return nil, unheld, errCacheClosed
	}
	if el, ok := c.entries[query]; ok {
		c.closeLocked(prepared)
		c.order.MoveToFront(el)
		stmt, release := c.heldLocked(el.Value.(*cacheEntry))
		return stmt, release, nil
	}
	e := &cacheEntry{sql: query, stmt: prepared}
	c.entries[query] = c.order.PushFront(e)
	if c.max > 0 && c.order.Len() > c.max {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		old := oldest.Value.(*cacheEntry)
		delete(c.entries, old.sql)
		if stmt := c.retireLocked(old); stmt != nil {
			c.closeLocked(stmt)
		}
	}
	stmt, release = c.heldLocked(e)
	return stmt, release, nil
}

func (c *StmtCache) closeLocked(stmt *sql.Stmt) {
	c.closing++
	go func() {
		_ = stmt.Close()
		c.mu.Lock()
		c.closing--
		if c.closing == 0 {
			c.drained.Broadcast()
		}
		c.mu.Unlock()
	}()
}

func (c *StmtCache) heldLocked(e *cacheEntry) (*sql.Stmt, func()) {
	e.users++
	var once sync.Once
	return e.stmt, func() { once.Do(func() { c.releaseOne(e) }) }
}

func (c *StmtCache) releaseOne(e *cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e.users--
	if e.gone && e.users == 0 {
		c.closeLocked(e.stmt)
	}
}

func (c *StmtCache) retireLocked(e *cacheEntry) *sql.Stmt {
	e.gone = true
	if e.users > 0 {
		return nil
	}
	return e.stmt
}

// Len reports the number of cached statements.
func (c *StmtCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// Close closes every cached statement, waits for the statements evicted
// before it to close, and rejects further use; one a caller still holds
// closes when it is released.
func (c *StmtCache) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	var idle []*sql.Stmt
	for el := c.order.Front(); el != nil; el = el.Next() {
		if stmt := c.retireLocked(el.Value.(*cacheEntry)); stmt != nil {
			idle = append(idle, stmt)
		}
	}
	c.order.Init()
	c.entries = map[string]*list.Element{}
	c.mu.Unlock()
	var errs []error
	for _, stmt := range idle {
		if err := stmt.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	c.mu.Lock()
	for c.closing > 0 {
		c.drained.Wait()
	}
	c.mu.Unlock()
	return errors.Join(errs...)
}
