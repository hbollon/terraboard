package db

import (
	"net/url"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/camptocamp/terraboard/internal/terraform/addrs"
	"github.com/camptocamp/terraboard/internal/terraform/states"
	"github.com/camptocamp/terraboard/internal/terraform/states/statefile"
	"github.com/camptocamp/terraboard/state"
	"github.com/camptocamp/terraboard/types"
)

func TestGetResourceIndex(t *testing.T) {
	tests := []struct {
		name string
		args addrs.InstanceKey
		want string
	}{
		{
			"StringKey",
			addrs.StringKey("module.bar"),
			"[\"module.bar\"]",
		},
		{
			"NoKey",
			addrs.NoKey,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getResourceIndex(tt.args)
			if got != tt.want {
				t.Errorf(
					"TestGetResourceIndex() -> \n\ngot:\n%v,\n\nwant:\n%v",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestMarshalAttributeValues(t *testing.T) {
	tests := []struct {
		name string
		args *states.ResourceInstanceObjectSrc
		want []types.Attribute
	}{
		{
			"Nil src",
			nil,
			nil,
		},
		{
			"Empty AttrsFlat",
			&states.ResourceInstanceObjectSrc{
				AttrsJSON: []byte(`{"ami":"bar"}`),
				Status:    states.ObjectReady,
			},
			[]types.Attribute{
				{
					Key:   "ami",
					Value: "\"bar\"",
				},
			},
		},
		{
			"Empty AttrsFlat with bad AttrsJSON JSON format",
			&states.ResourceInstanceObjectSrc{
				AttrsJSON: []byte(`"bar"`),
				Status:    states.ObjectReady,
			},
			nil,
		},
		{
			"With valid AttrsFlat",
			&states.ResourceInstanceObjectSrc{
				AttrsJSON: []byte(`{"ami":"bar"}`),
				AttrsFlat: map[string]string{
					"ami": "bar",
				},
				Status: states.ObjectReady,
			},
			[]types.Attribute{
				{
					Key:   "ami",
					Value: "\"bar\"",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := marshalAttributeValues(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf(
					"TestMarshalAttributeValues() -> \n\ngot:\n%v,\n\nwant:\n%v",
					got,
					tt.want,
				)
			}
		})
	}

}

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
		WillReturnRows(sqlmock.NewRows([]string{"version_id"}))

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

	// State retrieval
	mock.ExpectQuery(`^SELECT (.+) FROM "states" (.+)`).
		WithArgs("lineage", "foo").
		WillReturnRows(sqlmock.NewRows([]string{"id", "path"}).AddRow(1, `path`))
	mock.ExpectQuery(`^SELECT (.+) FROM "modules" (.+)`).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	db := &Database{
		DB: gormDB,
	}

	state := db.GetState("lineage", "foo")
	assert.NotNil(t, state)
	assert.Equal(t, uint(1), state.ID)
	assert.Equal(t, "path", state.Path)

	err = mock.ExpectationsWereMet()
	assert.Nil(t, err)
}

func TestGetLineageActivity(t *testing.T) {
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer fakeDB.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: fakeDB}))
	assert.Nil(t, err)

	// Lineage activity retrieval
	mock.ExpectQuery("^SELECT (.+)").
		WithArgs("lineage").
		WillReturnRows(sqlmock.NewRows([]string{"path", "version_id"}).AddRow("path", "foo"))

	db := &Database{
		DB: gormDB,
	}

	states := db.GetLineageActivity("lineage")
	assert.NotNil(t, states)
	assert.Equal(t, "path", states[0].Path)
	assert.Equal(t, "foo", states[0].VersionID)

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
		 LIMIT 1`)).
		WithArgs("foo").
		WillReturnRows(sqlmock.NewRows([]string{"version_id"}))

	mock.ExpectBegin()
	mock.ExpectQuery("^INSERT (.+)").
		WithArgs("foo", time.Time{}).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
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

func TestKnownVersions(t *testing.T) {
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer fakeDB.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: fakeDB}))
	assert.Nil(t, err)

	// Versions insertion
	mock.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "versions"
		 WHERE "versions"."version_id" = $1
		 ORDER BY "versions"."id"
		 LIMIT 1`)).
		WithArgs("foo").
		WillReturnRows(sqlmock.NewRows([]string{"version_id"}))

	mock.ExpectBegin()
	mock.ExpectQuery("^INSERT (.+)").
		WithArgs("foo", time.Time{}).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectCommit()

	// Known versions retrieval
	mock.ExpectQuery("^SELECT (.+)").
		WillReturnRows(sqlmock.NewRows([]string{"version_id"}).AddRow("foo"))

	db := &Database{
		DB: gormDB,
	}
	err = db.InsertVersion(&state.Version{
		ID: "foo",
	})
	assert.Nil(t, err)

	versions := db.KnownVersions()
	assert.Equal(t, 1, len(versions))
	assert.Equal(t, "foo", versions[0])

	err = mock.ExpectationsWereMet()
	assert.Nil(t, err)
}

func TestSearchAttribute(t *testing.T) {
	fakeDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer fakeDB.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: fakeDB,
	}))
	assert.Nil(t, err)

	// Search attribute
	mock.ExpectQuery("^SELECT count(.+)").
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
	mock.ExpectQuery("^SELECT (.+)").
		WithArgs("test_thing", "baz", "woozles", `"confuzles"`, 20).
		WillReturnRows(sqlmock.NewRows([]string{"path", "version_id", "tf_version"}).AddRow("path", "foo", "1.0.0"))

	db := &Database{
		DB: gormDB,
	}

	params := url.Values{}
	params.Add("name", "baz")
	params.Add("type", "test_thing")
	params.Add("key", "woozles")
	params.Add("value", `"confuzles"`)
	params.Add("tf_version", "1.0.0")

	results, page, total := db.SearchAttribute(params)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, 1, page)
	assert.Equal(t, 1, total)
	assert.Equal(t, "path", results[0].Path)
	assert.Equal(t, "1.0.0", results[0].TFVersion)
	assert.Equal(t, "foo", results[0].VersionID)

	err = mock.ExpectationsWereMet()
	assert.Nil(t, err)
}
