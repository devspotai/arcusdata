package psqldb

import (
	"sync"

	"github.com/devspotai/arcusdata/psqldb/dbinterface"
)

// LazyDatabaseInterface defines the interface for lazy database initialization
type LazyDatabaseInterface interface {
	Get() (dbinterface.Database, error)
	Close() error
	Reset()
}

type LazyDatabase struct {
	mu       sync.Mutex
	once     sync.Once
	db       dbinterface.Database
	err      error
	provider dbinterface.DBProvider
	closed   bool
}

func NewLazyDatabase(provider dbinterface.DBProvider) LazyDatabaseInterface {
	return &LazyDatabase{
		provider: provider,
	}
}

// Lazily initializes the database (once)
func (l *LazyDatabase) Get() (dbinterface.Database, error) {
	l.once.Do(func() {
		l.db, l.err = l.provider()
	})
	return l.db, l.err
}

// Closes the database if initialized
func (l *LazyDatabase) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}

	if l.db != nil {
		l.db.Close()
		l.db = nil
	}
	l.closed = true
	return nil
}

// Resets the pool so it can be re-initialized
func (l *LazyDatabase) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Close the current pool if it was initialized
	if l.db != nil && !l.closed {
		l.db.Close()
	}

	// Reset state
	l.once = sync.Once{}
	l.db = nil
	l.err = nil
	l.closed = false
}

// Compile-time check to ensure LazyDatabase implements LazyDatabaseInterface
var _ LazyDatabaseInterface = (*LazyDatabase)(nil)
