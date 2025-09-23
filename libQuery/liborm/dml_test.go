package liborm_test

import (
	"context"
	"testing"

	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libQuery/liborm"
	"github.com/hmmftg/requestCore/libQuery/mockdb"
	"gorm.io/gorm"
)

func TestSetVariable(t *testing.T) {
	helper := mockdb.NewMockDBHelper(t)
	defer helper.Close()

	// Set up mock expectations for set_config query - use flexible matching
	helper.ExpectExecFlexible()

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
				tx:      helper.DB,
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
	helper := mockdb.NewMockDBHelper(t)
	defer helper.Close()

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
				DB:          helper.DB,
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
	helper := mockdb.NewMockDBHelper(t)
	defer helper.Close()

	// Set up mock expectations for set_config query - use flexible matching
	// Expect 4 exec calls for APP, USER, MODULE, METHOD
	helper.ExpectExecFlexible() // APP
	helper.ExpectExecFlexible() // USER
	helper.ExpectExecFlexible() // MODULE
	helper.ExpectExecFlexible() // METHOD

	type args struct {
		ctx        context.Context
		ormModel   *liborm.OrmModel
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
				ormModel:   helper.GetOrmModel(),
				moduleName: "test",
				methodName: "test",
				tx:         helper.DB,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.ormModel.SetModifVariables(tt.args.ctx, tt.args.moduleName, tt.args.methodName, tt.args.tx)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetModifVariables() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDml(t *testing.T) {
	helper := mockdb.NewMockDBHelper(t)
	defer helper.Close()

	// Set up transaction expectations
	helper.ExpectBegin()
	// Expect multiple exec calls for SetModifVariables (4 calls) + 1 for the actual DML
	helper.ExpectExecFlexible()                                         // APP
	helper.ExpectExecFlexible()                                         // USER
	helper.ExpectExecFlexible()                                         // MODULE
	helper.ExpectExecFlexible()                                         // METHOD
	helper.ExpectExec("INSERT INTO table \\(name\\) VALUES \\(\\$1\\)") // Actual DML
	helper.ExpectCommit()

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
			_, err := helper.GetOrmModel().Dml(tt.args.ctx, tt.args.moduleName, tt.args.methodName, tt.args.command, tt.args.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dml() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
