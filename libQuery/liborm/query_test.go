package liborm_test

import (
	"testing"

	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libQuery/liborm"
	"github.com/hmmftg/requestCore/libQuery/mockdb"
	"gorm.io/gorm"
)

// SimpleTestStruct represents a simplified test structure
type SimpleTestStruct struct {
	ID    int    `db:"id"`
	Name  string `db:"name"`
	Value string `db:"value"`
}

func (s SimpleTestStruct) GetID() string { return "" }
func (s SimpleTestStruct) GetValue() any { return "" }

func TestGetQuery(t *testing.T) {
	helper := mockdb.NewMockDBHelper(t)
	defer helper.Close()

	// Set up mock query expectation
	helper.ExpectQuery("SELECT \\* FROM table", []string{"id", "name", "value"}, 1, "test", "value")

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
				core:  helper.GetOrmModel(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := liborm.GetQuery[SimpleTestStruct](tt.args.query, tt.args.core)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetQuery() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQueryToStruct(t *testing.T) {
	helper := mockdb.NewMockDBHelper(t)
	defer helper.Close()

	// Set up mock query expectation
	helper.ExpectQuery("SELECT \\* FROM table", []string{"id", "name", "value"}, 1, "test", "value")

	type args struct {
		db    *gorm.DB
		query string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test query to struct",
			args: args{
				db:    helper.DB,
				query: "SELECT * FROM table",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := liborm.QueryToStruct[SimpleTestStruct](tt.args.db, tt.args.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("QueryToStruct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQuery(t *testing.T) {
	helper := mockdb.NewMockDBHelper(t)
	defer helper.Close()

	// Set up mock query expectation
	helper.ExpectQuery("SELECT \\* FROM table", []string{"id", "name", "value"}, 1, "test", "value")

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
				core: helper.GetOrmModel(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := liborm.Query[SimpleTestStruct](tt.args.command, tt.args.core)
			if (err != nil) != tt.wantErr {
				t.Errorf("Query() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
