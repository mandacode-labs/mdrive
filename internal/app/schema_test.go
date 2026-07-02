//go:build !integration

package app

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/mdrive/internal/errorx"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

func TestVerifySchemaProductionFailFastOnMissingColumn(t *testing.T) {
	db, mock := newMockDB(t)
	for i := 0; i < len(requiredColumns)-1; i++ {
		c := requiredColumns[i]
		mock.ExpectQuery(`SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`).
			WithArgs(c.Table, c.Name).
			WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type"}).
				AddRow(c.Name, c.Type))
	}
	last := requiredColumns[len(requiredColumns)-1]
	mock.ExpectQuery(`SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`).
		WithArgs(last.Table, last.Name).
		WillReturnError(sql.ErrNoRows)

	err := verifySchema(context.Background(), db, "production")
	require.Error(t, err, "production must fail-fast on missing column")
	var de errorx.Error
	require.True(t, errors.As(err, &de), "error must be errorx.Error")
	assert.Equal(t, errorx.KindServiceDegraded, de.Kind())
	assert.Contains(t, err.Error(), "missing column")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifySchemaProductionFailFastOnWrongType(t *testing.T) {
	db, mock := newMockDB(t)
	c := requiredColumns[0]
	mock.ExpectQuery(`SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`).
		WithArgs(c.Table, c.Name).
		WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type"}).
			AddRow(c.Name, "text"))

	err := verifySchema(context.Background(), db, "production")
	require.Error(t, err, "production must fail-fast on wrong type")
	assert.Contains(t, err.Error(), "wrong type")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifySchemaDevelopmentWarnsOnMissingColumn(t *testing.T) {
	db, mock := newMockDB(t)
	for i := 0; i < len(requiredColumns); i++ {
		c := requiredColumns[i]
		if i == 0 {
			mock.ExpectQuery(`SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`).
				WithArgs(c.Table, c.Name).
				WillReturnError(sql.ErrNoRows)
			continue
		}
		mock.ExpectQuery(`SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`).
			WithArgs(c.Table, c.Name).
			WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type"}).
				AddRow(c.Name, c.Type))
	}

	err := verifySchema(context.Background(), db, "development")
	assert.NoError(t, err, "development must not fail on missing column; it logs a warning and continues")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifySchemaAllPresent(t *testing.T) {
	db, mock := newMockDB(t)
	for _, c := range requiredColumns {
		mock.ExpectQuery(`SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1 AND column_name = $2`).
			WithArgs(c.Table, c.Name).
			WillReturnRows(sqlmock.NewRows([]string{"column_name", "data_type"}).
				AddRow(c.Name, c.Type))
	}
	err := verifySchema(context.Background(), db, "production")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
