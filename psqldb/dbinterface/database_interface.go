package dbinterface

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=./database_interface.go -destination=mocks/mock_database.go -package=mocks

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database interface {
	Close()                            // Implemented
	Pool() *pgxpool.Pool               // Implemented
	Ping(ctx context.Context) error    // Implemented
	Stats() *pgxpool.Stat              // Implemented
	PoolStats() map[string]interface{} // Implemented

	BeginTx(ctx context.Context, opts pgx.TxOptions) (Transaction, error)                  // Implemented
	Begin(ctx context.Context) (Transaction, error)                                        // Implemented
	BeginFunc(ctx context.Context, fn func(Transaction) error) error                       // Implemented
	BeginTxFunc(ctx context.Context, opts pgx.TxOptions, fn func(Transaction) error) error // Implemented

	Exec(ctx context.Context, sql string, args ...interface{}) (CommandTag, error)                                          // Implemented
	Query(ctx context.Context, sql string, args ...interface{}) (Rows, error)                                               // Implemented
	QueryRow(ctx context.Context, sql string, args ...interface{}) Row                                                      // Implemented
	SendBatch(ctx context.Context, b *pgx.Batch) BatchResults                                                               // Implemented
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) // Implemented

	NewBatchOperation() BatchOperation                         // Implemented
	ExecMigration(ctx context.Context, migration string) error // Implemented
}

type Transaction interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) Row
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	IsClosed() bool
}

type Rows interface {
	Close()
	Err() error
	Next() bool
	Scan(dest ...interface{}) error
}

// CommandTag interface for exec results
type CommandTag interface {
	RowsAffected() int64
}

type BatchOperation interface {
	Queue(query string, args ...interface{})
	Execute(ctx context.Context) error
}

type Row interface {
	Scan(dest ...interface{}) error
}

type Batch interface {
	Queue(query string, arguments ...any)
	Len() int
	Underlying() *pgx.Batch
}

type BatchResults interface {
	Exec() (CommandTag, error)
	Query() (Rows, error)
	QueryRow() Row
	Close() error
}

type DBProvider func() (Database, error)
