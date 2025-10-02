package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// QueryBuilder helps build SQL queries safely
type QueryBuilder struct {
	query  strings.Builder
	args   []interface{}
	argIdx int
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		args:   make([]interface{}, 0),
		argIdx: 0,
	}
}

// Select adds a SELECT clause
func (qb *QueryBuilder) Select(columns ...string) *QueryBuilder {
	qb.query.WriteString("SELECT ")
	qb.query.WriteString(strings.Join(columns, ", "))
	return qb
}

// From adds a FROM clause
func (qb *QueryBuilder) From(table string) *QueryBuilder {
	qb.query.WriteString(" FROM ")
	qb.query.WriteString(table)
	return qb
}

// Where adds a WHERE clause
func (qb *QueryBuilder) Where(condition string, args ...interface{}) *QueryBuilder {
	qb.query.WriteString(" WHERE ")
	qb.query.WriteString(condition)
	qb.args = append(qb.args, args...)
	return qb
}

// And adds an AND condition
func (qb *QueryBuilder) And(condition string, args ...interface{}) *QueryBuilder {
	qb.query.WriteString(" AND ")
	qb.query.WriteString(condition)
	qb.args = append(qb.args, args...)
	return qb
}

// Or adds an OR condition
func (qb *QueryBuilder) Or(condition string, args ...interface{}) *QueryBuilder {
	qb.query.WriteString(" OR ")
	qb.query.WriteString(condition)
	qb.args = append(qb.args, args...)
	return qb
}

// OrderBy adds an ORDER BY clause
func (qb *QueryBuilder) OrderBy(column string, desc bool) *QueryBuilder {
	qb.query.WriteString(" ORDER BY ")
	qb.query.WriteString(column)
	if desc {
		qb.query.WriteString(" DESC")
	} else {
		qb.query.WriteString(" ASC")
	}
	return qb
}

// Limit adds a LIMIT clause
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.query.WriteString(fmt.Sprintf(" LIMIT %d", limit))
	return qb
}

// Offset adds an OFFSET clause
func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	qb.query.WriteString(fmt.Sprintf(" OFFSET %d", offset))
	return qb
}

// Build returns the built query and arguments
func (qb *QueryBuilder) Build() (string, []interface{}) {
	return qb.query.String(), qb.args
}

// ScanRow is a helper to scan a single row into a struct
func ScanRow(row *sql.Row, dest ...interface{}) error {
	return row.Scan(dest...)
}

// ScanRows is a helper to scan multiple rows
func ScanRows(rows *sql.Rows, scanFunc func(*sql.Rows) error) error {
	defer rows.Close()
	
	for rows.Next() {
		if err := scanFunc(rows); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
	}
	
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows error: %w", err)
	}
	
	return nil
}

// QueryOne executes a query that returns a single row
func QueryOne(ctx context.Context, db *sql.DB, query string, args []interface{}, scanFunc func(*sql.Row) error) error {
	row := db.QueryRowContext(ctx, query, args...)
	return scanFunc(row)
}

// QueryMany executes a query that returns multiple rows
func QueryMany(ctx context.Context, db *sql.DB, query string, args []interface{}, scanFunc func(*sql.Rows) error) error {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}
	
	return ScanRows(rows, scanFunc)
}

// BulkInsert performs a bulk insert operation
func BulkInsert(ctx context.Context, db *sql.DB, table string, columns []string, values [][]interface{}) error {
	if len(values) == 0 {
		return nil
	}

	// Build the query
	var queryBuilder strings.Builder
	queryBuilder.WriteString(fmt.Sprintf("INSERT INTO %s (%s) VALUES ", table, strings.Join(columns, ", ")))

	var args []interface{}
	placeholders := make([]string, 0, len(values))
	
	for i, row := range values {
		rowPlaceholders := make([]string, len(columns))
		for j := range columns {
			args = append(args, row[j])
			rowPlaceholders[j] = fmt.Sprintf("$%d", i*len(columns)+j+1)
		}
		placeholders = append(placeholders, fmt.Sprintf("(%s)", strings.Join(rowPlaceholders, ", ")))
	}

	queryBuilder.WriteString(strings.Join(placeholders, ", "))
	
	_, err := db.ExecContext(ctx, queryBuilder.String(), args...)
	return err
}