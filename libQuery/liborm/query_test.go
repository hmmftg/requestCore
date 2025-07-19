package liborm_test

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libQuery/liborm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestGetQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db, DriverName: "postgres"}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT \\* FROM table").
		WillReturnRows(sqlmock.NewRows([]string{"column"}).AddRow("value"))

	type args struct {
		query string
		core  liborm.OrmInterface
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test get query",
			args: args{
				query: "SELECT * FROM table",
				core:  &liborm.OrmModel{DB: gormDB},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := liborm.GetQuery[interface{}](tt.args.query, tt.args.core)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetQuery() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQueryToStruct(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db, DriverName: "postgres"}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT \\* FROM table").
		WillReturnRows(sqlmock.NewRows([]string{"column"}).AddRow("value"))
	type Sample struct {
		Column string
	}

	type args struct {
		q      *gorm.DB
		query  string
		args   []any
		target *Sample
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test query to struct",
			args: args{
				q:      gormDB,
				query:  "SELECT * FROM table",
				target: &Sample{},
			},
			wantErr: false,
		},
	}

	type sample struct {
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := liborm.QueryToStruct[sample](tt.args.q, tt.args.query, tt.args.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("QueryToStruct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db, DriverName: "postgres"}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT \\* FROM table").
		WillReturnRows(sqlmock.NewRows([]string{"column"}).AddRow("value"))

	type args struct {
		command libQuery.CommandInterface
		core    liborm.OrmInterface
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test query",
			args: args{
				command: &libQuery.QueryCommand{
					Name:    "",
					Command: "SELECT * FROM table",
					Type:    libQuery.QuerySingle,
				},
				core: &liborm.OrmModel{DB: gormDB},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := liborm.Query[interface{}](tt.args.command, tt.args.core)
			if (err != nil) != tt.wantErr {
				t.Errorf("Query() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
