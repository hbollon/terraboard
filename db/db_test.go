package db

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/camptocamp/terraboard/internal/terraform/states"
	"github.com/camptocamp/terraboard/internal/terraform/states/statefile"
	"github.com/camptocamp/terraboard/state"
)

func TestInsertState(t *testing.T) {
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer fakeDB.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: fakeDB}))
	assert.Nil(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "versions"
		 WHERE "versions"."version_id" = $1
		 ORDER BY "versions"."id"
		 LIMIT 1`,
	)).
		WithArgs("foo").
		WillReturnRows(sqlmock.NewRows([]string{"version_id"})).
		WillReturnError(gorm.ErrRecordNotFound)

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "lineages" 
		 WHERE "lineages"."value" = $1 AND "lineages"."deleted_at" IS NULL 
		 ORDER BY "lineages"."id" 
		 LIMIT 1`,
	)).WithArgs("lineage").WillReturnRows(sqlmock.NewRows([]string{"id"}))

	mock.ExpectBegin()
	mock.ExpectQuery("^INSERT (.+)").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), nil, "lineage").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery("^INSERT (.+)").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), nil, "path", nil, "1.0.0", 2, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	db := &Database{
		DB: gormDB,
	}

	version, err := version.NewSemver("v1.0.0")
	err = db.InsertState("path", "foo", &statefile.File{
		TerraformVersion: version,
		Serial:           2,
		Lineage:          "lineage",
		State:            &states.State{},
	})

	assert.Nil(t, err)
	err = mock.ExpectationsWereMet()
	assert.Nil(t, err)
}

func TestGetState(t *testing.T) {
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer fakeDB.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: fakeDB}))
	assert.Nil(t, err)

	// State insertion
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "versions"
		 WHERE "versions"."version_id" = $1
		 ORDER BY "versions"."id"
		 LIMIT 1`,
	)).
		WithArgs("foo").
		WillReturnRows(sqlmock.NewRows([]string{"id", "version_id"}).AddRow(1, "foo"))

	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "lineages" 
		 WHERE "lineages"."value" = $1 AND "lineages"."deleted_at" IS NULL 
		 ORDER BY "lineages"."id" 
		 LIMIT 1`,
	)).
		WithArgs("lineage").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	mock.ExpectBegin()
	mock.ExpectQuery(`^INSERT INTO "lineages" (.+)`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), nil, "lineage").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectQuery(`^INSERT INTO "versions" (.+)`).
		WithArgs("foo", time.Time{}, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`^INSERT INTO "states" (.+)`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), nil, "path", 1, "1.0.0", 2, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	// State retrieval
	mock.ExpectQuery("^SELECT (.+)").
		WithArgs("lineage", "foo").
		WillReturnRows(sqlmock.NewRows([]string{"id", "path"}).AddRow(1, "path"))
	mock.ExpectQuery("^SELECT (.+)").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{}))

	db := &Database{
		DB: gormDB,
	}

	tfVersion, err := version.NewSemver("v1.0.0")
	err = db.InsertState("path", "foo", &statefile.File{
		TerraformVersion: tfVersion,
		Serial:           2,
		Lineage:          "lineage",
		State:            &states.State{},
	})
	assert.Nil(t, err)

	state := db.GetState("lineage", "foo")
	assert.NotNil(t, state)
	assert.Equal(t, uint(1), state.ID)
	assert.Equal(t, "path", state.Path)

	err = mock.ExpectationsWereMet()
	assert.Nil(t, err)
}

func TestInsertVersionCreated(t *testing.T) {
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer fakeDB.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: fakeDB}))
	assert.Nil(t, err)

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
