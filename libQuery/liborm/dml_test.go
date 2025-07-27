package liborm_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libQuery/liborm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSetVariable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db, DriverName: "postgres"}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectExec("SELECT set_config\\$\\$1,\\$2, true\\$;").
		WillReturnResult(sqlmock.NewResult(1, 1))

	type args struct {
		ctx     context.Context
		tx      *gorm.DB
		command string
		key     string
		value   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test set variable",
			args: args{
				ctx:     context.Background(),
				tx:      gormDB,
				command: "SELECT set_config($1,$2,true);",
				key:     "application_name",
				value:   "test",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := liborm.SetVariable(tt.args.ctx, tt.args.tx, tt.args.command, tt.args.key, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetVariable() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInit(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db, DriverName: "postgres"}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	type args struct {
		DB          *gorm.DB
		ProgramName string
		ModuleName  string
		mode        libQuery.DBMode
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test init",
			args: args{
				DB:          gormDB,
				ProgramName: "test",
				ModuleName:  "test",
				mode:        libQuery.Oracle,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := liborm.Init(tt.args.DB, tt.args.ProgramName, tt.args.ModuleName, tt.args.mode)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetModifVariables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db, DriverName: "postgres"}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectExec("SELECT set_config\\$\\$1,\\$2, true\\$;").
		WillReturnResult(sqlmock.NewResult(1, 1))

	type args struct {
		ctx        context.Context
		moduleName string
		methodName string
		tx         *gorm.DB
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test set modification variables",
			args: args{
				ctx:        context.Background(),
				moduleName: "test",
				methodName: "test",
				tx:         gormDB,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&liborm.OrmModel{DB: gormDB}).SetModifVariables(tt.args.ctx, tt.args.moduleName, tt.args.methodName, tt.args.tx)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetModifVariables() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDml(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db, DriverName: "postgres"}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectExec("INSERT INTO table $name$ VALUES $\\?$").
		WillReturnResult(sqlmock.NewResult(1, 1))

	type args struct {
		ctx        context.Context
		moduleName string
		methodName string
		command    string
		args       []any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test dml",
			args: args{
				ctx:        context.Background(),
				moduleName: "test",
				methodName: "test",
				command:    "INSERT INTO table (name) VALUES (?)",
				args:       []any{"test"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&liborm.OrmModel{DB: gormDB}).Dml(tt.args.ctx, tt.args.moduleName, tt.args.methodName, tt.args.command, tt.args.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dml() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
