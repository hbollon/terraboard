package db

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/camptocamp/terraboard/state"
)

func TestInsertVersionCreated(t *testing.T) {
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer fakeDB.Close()
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: fakeDB}))
	assert.Nil(t, err)

	mock.MatchExpectationsInOrder(true)
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "versions"
		 WHERE "versions"."version_id" = $1
		 ORDER BY "versions"."id"
		 LIMIT 1`)).WithArgs("foo").WillReturnRows(sqlmock.NewRows([]string{"version_id"}))
	mock.ExpectBegin()
	mock.ExpectQuery("^INSERT (.+)").WithArgs("foo", time.Time{}).WillReturnRows(sqlmock.NewRows([]string{}))
	mock.ExpectCommit()

	db := &Database{
		DB: gormDB,
	}
	err = db.InsertVersion(&state.Version{
		ID: "foo",
	})
	assert.Nil(t, err)
	err = mock.ExpectationsWereMet()
	assert.Nil(t, err)
}
