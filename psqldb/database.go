package psqldb

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/devspotai/arcusdata/psqldb/dbinterface"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PgxDatabase struct {
	pool *pgxpool.Pool
}

// Tx represents a database transaction
type PgxTransaction struct {
	tx     pgx.Tx
	ctx    context.Context
	db     *PgxDatabase
	closed bool
}

type PgxBatchOperation struct {
	db    dbinterface.Database
	batch dbinterface.Batch
}

// Database defines the interface for database operations.

type PgxRows struct {
	rows pgx.Rows
}

func (r *PgxRows) Close() {
	r.rows.Close()
}

func (r *PgxRows) Err() error {
	return r.rows.Err()
}

func (r *PgxRows) Next() bool {
	return r.rows.Next()
}

func (r *PgxRows) Scan(dest ...interface{}) error {
	return r.rows.Scan(dest...)
}

type PgxCommandTag struct {
	tag pgconn.CommandTag
}

func (t PgxCommandTag) RowsAffected() int64 {
	return t.tag.RowsAffected()
}

// Row adapters
type PgxRow struct {
	row pgx.Row
}

func (r *PgxRow) Scan(dest ...interface{}) error {
	return r.row.Scan(dest...)
}

func NewPgxDatabaseProvider(url string, connectionPoolConfig ConnectionPoolConfig) dbinterface.DBProvider {
	return func() (dbinterface.Database, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		config, err := pgxpool.ParseConfig(url)
		if err != nil {
			log.Fatalf("Unable to parse database URL: %v", err)
		}
		// Connection Pool Settings
		config.MaxConns = int32(connectionPoolConfig.MaxOpenConns)                                          // Max open connections (default is unlimited)
		config.MinConns = int32(connectionPoolConfig.MaxIdleConns)                                          // Max idle connections
		config.MaxConnLifetime = time.Duration(connectionPoolConfig.ConnMaxLifetimeInHours) * time.Hour     // Max connection lifetime
		config.MaxConnIdleTime = time.Duration(connectionPoolConfig.ConnMaxIdleTimeInMin) * time.Minute     // Max connection idle time
		config.HealthCheckPeriod = time.Duration(connectionPoolConfig.HealthCheckPeriodInSec) * time.Second // Health check period
		config.BeforeAcquire = func(ctx context.Context, conn *pgx.Conn) bool {
			var ok bool
			err := conn.QueryRow(ctx, "SELECT true").Scan(&ok)
			if err != nil || !ok {
				log.Println("Connection health check failed")
				return false
			}
			log.Println("Acquiring a connection from the pool")
			return true
		}
		config.AfterRelease = func(conn *pgx.Conn) bool {
			// Can add logging or metrics here
			return true
		}
		// Create connection pool
		dbpool, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			log.Fatalf("Unable to create database connection pool: %v", err)
			return nil, fmt.Errorf("failed to create connection pool: %w", err)
		}

		// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		// 	defer cancel()
		if err := dbpool.Ping(ctx); err != nil {
			dbpool.Close()
			log.Fatalf("Database connection failed: %v", err)
			return nil, fmt.Errorf("failed to ping database: %w", err)
		}
		return &PgxDatabase{pool: dbpool}, nil
	}
}

// CloseDB closes the database connection pool
func (db *PgxDatabase) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}

// Pool returns the underlying connection pool
func (db *PgxDatabase) Pool() *pgxpool.Pool {
	return db.pool
}

// Ping verifies a connection to the database is still alive
func (db *PgxDatabase) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// Stats returns connection pool statistics
func (db *PgxDatabase) Stats() *pgxpool.Stat {
	return db.pool.Stat()
}

func (db *PgxDatabase) PoolStats() map[string]interface{} {
	stats := db.pool.Stat()
	return map[string]interface{}{
		"total_conns":        stats.TotalConns(),
		"acquired_conns":     stats.AcquiredConns(),
		"max_conns":          stats.MaxConns(),
		"constructing_conns": stats.ConstructingConns(),
		"idle_conns":         stats.IdleConns(),
	}
}

// BeginTx starts a new transaction with the given options
func (db *PgxDatabase) BeginTx(ctx context.Context, opts pgx.TxOptions) (dbinterface.Transaction, error) {
	tx, err := db.pool.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	return &PgxTransaction{
		tx:     tx,
		ctx:    ctx,
		db:     db,
		closed: false,
	}, nil
}

// Begin starts a new transaction with default options
func (db *PgxDatabase) Begin(ctx context.Context) (dbinterface.Transaction, error) {
	return db.BeginTx(ctx, pgx.TxOptions{})
}

// BeginFunc starts a new transaction and executes the given function
// The transaction is automatically committed if the function returns nil,
// or rolled back if the function returns an error
func (db *PgxDatabase) BeginFunc(ctx context.Context, fn func(dbinterface.Transaction) error) error {
	return db.BeginTxFunc(ctx, pgx.TxOptions{}, fn)
}

// BeginTxFunc starts a new transaction with options and executes the given function
func (db *PgxDatabase) BeginTxFunc(ctx context.Context, opts pgx.TxOptions, fn func(dbinterface.Transaction) error) error {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}

	defer func() {
		if !tx.IsClosed() {
			_ = tx.Rollback(context.Background())
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("tx failed: %v, rollback failed: %w", err, rbErr)
		}
		return err
	}
	return tx.Commit(ctx)
}

// Exec executes a query without returning any rows
func (db *PgxDatabase) Exec(ctx context.Context, sql string, args ...interface{}) (dbinterface.CommandTag, error) {
	tag, err := db.pool.Exec(ctx, sql, args...)
	return PgxCommandTag{tag: tag}, err
}

// Query executes a query that returns rows
func (db *PgxDatabase) Query(ctx context.Context, sql string, args ...interface{}) (dbinterface.Rows, error) {
	rows, err := db.pool.Query(ctx, sql, args...)
	return &PgxRows{rows: rows}, err
}

// QueryRow executes a query that returns at most one row
func (db *PgxDatabase) QueryRow(ctx context.Context, sql string, args ...interface{}) dbinterface.Row {
	row := db.pool.QueryRow(ctx, sql, args...)
	return &PgxRow{row: row}
}

// SendBatch sends a batch of queries to be executed
func (db *PgxDatabase) SendBatch(ctx context.Context, b *pgx.Batch) dbinterface.BatchResults {
	return &PgxBatchResults{br: db.pool.SendBatch(ctx, b)}
}

// BatchResults defines an interface for batch results

// PgxBatchResults adapts pgx.BatchResults to BatchResults interface
type PgxBatchResults struct {
	br pgx.BatchResults
}

func (r *PgxBatchResults) Exec() (dbinterface.CommandTag, error) {
	tag, err := r.br.Exec()
	return &PgxCommandTag{tag: tag}, err
}

func (r *PgxBatchResults) Query() (dbinterface.Rows, error) {
	rows, err := r.br.Query()
	return &PgxRows{rows: rows}, err
}

func (r *PgxBatchResults) QueryRow() dbinterface.Row {
	row := r.br.QueryRow()
	return &PgxRow{row: row}
}

func (r *PgxBatchResults) Close() error {
	return r.br.Close()
}

///////////////////////////////////

// CopyFrom performs a bulk copy operation
func (db *PgxDatabase) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return db.pool.CopyFrom(ctx, tableName, columnNames, rowSrc)
}

// Transaction isolation levels
// TxReadCommitted provides transaction options for the "Read Committed" isolation level.
var (
	TxReadCommitted = pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	}
	// TxRepeatableRead provides transaction options for the "Repeatable Read" isolation level.
	TxRepeatableRead = pgx.TxOptions{
		IsoLevel: pgx.RepeatableRead,
	}
	// TxSerializable provides transaction options for the "Serializable" isolation level.
	TxSerializable = pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	}
)

func (t *PgxTransaction) Exec(ctx context.Context, sql string, arguments ...interface{}) (dbinterface.CommandTag, error) {
	tag, err := t.tx.Exec(ctx, sql, arguments...)
	return &PgxCommandTag{tag: tag}, err
}

func (t *PgxTransaction) Query(ctx context.Context, sql string, args ...interface{}) (dbinterface.Rows, error) {
	rows, err := t.tx.Query(ctx, sql, args...)
	return &PgxRows{rows: rows}, err
}

func (t *PgxTransaction) QueryRow(ctx context.Context, sql string, args ...interface{}) dbinterface.Row {
	return &PgxRow{row: t.tx.QueryRow(ctx, sql, args...)}
}

// Commit commits the transaction
func (tx *PgxTransaction) Commit(ctx context.Context) error {
	if tx.closed {
		return fmt.Errorf("transaction already closed")
	}
	tx.closed = true
	return tx.tx.Commit(ctx)
}

// Rollback rolls back the transaction
func (tx *PgxTransaction) Rollback(ctx context.Context) error {
	if tx.closed {
		return nil // Already closed, nothing to do
	}
	tx.closed = true
	return tx.tx.Rollback(ctx)
}

// Rollback rolls back the transaction
func (tx *PgxTransaction) IsClosed() bool {
	return tx.closed
}

// Error handling helpers

// IsNoRows checks if an error is a "no rows" error
func IsNoRows(err error) bool {
	return err == pgx.ErrNoRows
}

// IsUniqueViolation checks if an error is a unique constraint violation
func IsUniqueViolation(err error) bool {
	if pgErr, ok := err.(*pgconn.PgError); ok {
		return pgErr.Code == "23505"
	}
	return false
}

// IsForeignKeyViolation checks if an error is a foreign key violation
func IsForeignKeyViolation(err error) bool {
	if pgErr, ok := err.(*pgconn.PgError); ok {
		return pgErr.Code == "23503"
	}
	return false
}

// IsCheckViolation checks if an error is a check constraint violation
func IsCheckViolation(err error) bool {
	if pgErr, ok := err.(*pgconn.PgError); ok {
		return pgErr.Code == "23514"
	}
	return false
}

// GetConstraintName extracts the constraint name from a constraint violation error
func GetConstraintName(err error) string {
	if pgErr, ok := err.(*pgconn.PgError); ok {
		return pgErr.ConstraintName
	}
	return ""
}

type PgxBatch struct {
	batch *pgx.Batch
}

func NewPgxBatch() *PgxBatch {
	return &PgxBatch{
		batch: &pgx.Batch{},
	}
}

func (b *PgxBatch) Queue(query string, args ...interface{}) {
	b.batch.Queue(query, args...)
}

func (b *PgxBatch) Len() int {
	return b.batch.Len()
}

func (b *PgxBatch) Underlying() *pgx.Batch {
	return b.batch
}

// Batch operations helper
type BatchOperation struct {
	db    dbinterface.Database
	batch dbinterface.Batch
}

// NewBatchOperation creates a new batch operation
func (db *PgxDatabase) NewBatchOperation() dbinterface.BatchOperation {
	return &BatchOperation{
		db:    db,
		batch: NewPgxBatch(),
	}
}

// Queue adds a query to the batch
func (bo *BatchOperation) Queue(query string, args ...interface{}) {
	bo.batch.Queue(query, args...)
}

// Execute executes all queued queries in the batch
func (bo *BatchOperation) Execute(ctx context.Context) error {
	br := bo.db.SendBatch(ctx, bo.batch.Underlying())
	defer br.Close()

	// Process all results
	for i := 0; i < bo.batch.Len(); i++ {
		_, err := br.Exec()
		if err != nil {
			return fmt.Errorf("batch query %d failed: %w", i, err)
		}
	}

	return nil
}

// Migration helper
func (db *PgxDatabase) ExecMigration(ctx context.Context, migration string) error {
	return db.BeginFunc(ctx, func(tx dbinterface.Transaction) error {
		_, err := tx.Exec(ctx, migration)
		return err
	})
}

func convertQuestionMarksToDollarPlaceholders(query string) string {
	var i int
	var result strings.Builder

	for _, r := range query {
		if r == '?' {
			i++
			result.WriteString(fmt.Sprintf("$%d", i))
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}
