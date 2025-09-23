package mockdb

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libQuery/liborm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// MockDBHelper provides a simplified interface for creating mock databases
type MockDBHelper struct {
	DB    *gorm.DB
	Mock  sqlmock.Sqlmock
	RawDB *sql.DB
}

// NewMockDBHelper creates a new mock database helper
func NewMockDBHelper(t *testing.T) *MockDBHelper {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db, DriverName: "postgres"}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	return &MockDBHelper{
		DB:    gormDB,
		Mock:  mock,
		RawDB: db,
	}
}

// Close closes the mock database connection
func (m *MockDBHelper) Close() {
	m.RawDB.Close()
}

// ExpectQuery sets up a mock query expectation
func (m *MockDBHelper) ExpectQuery(sqlRegex string, columns []string, values ...driver.Value) {
	rows := sqlmock.NewRows(columns)
	if len(values) > 0 {
		rows.AddRow(values...)
	}
	m.Mock.ExpectQuery(sqlRegex).WillReturnRows(rows)
}

// ExpectExec sets up a mock execution expectation with flexible matching
func (m *MockDBHelper) ExpectExec(sqlRegex string) {
	m.Mock.ExpectExec(sqlRegex).WillReturnResult(sqlmock.NewResult(1, 1))
}

// ExpectExecFlexible sets up a mock execution expectation that matches any SQL
func (m *MockDBHelper) ExpectExecFlexible() {
	m.Mock.ExpectExec(".*").WillReturnResult(sqlmock.NewResult(1, 1))
}

// ExpectBegin sets up a mock transaction begin expectation
func (m *MockDBHelper) ExpectBegin() {
	m.Mock.ExpectBegin()
}

// ExpectCommit sets up a mock transaction commit expectation
func (m *MockDBHelper) ExpectCommit() {
	m.Mock.ExpectCommit()
}

// ExpectRollback sets up a mock transaction rollback expectation
func (m *MockDBHelper) ExpectRollback() {
	m.Mock.ExpectRollback()
}

// ExpectPrepare sets up a mock prepared statement expectation
func (m *MockDBHelper) ExpectPrepare(sqlRegex string) {
	m.Mock.ExpectPrepare(sqlRegex)
}

// GetOrmModel returns an OrmModel with the mock database
func (m *MockDBHelper) GetOrmModel() *liborm.OrmModel {
	return &liborm.OrmModel{DB: m.DB}
}

// GetQueryRunnerModel returns a QueryRunnerModel with the mock database
func (m *MockDBHelper) GetQueryRunnerModel() libQuery.QueryRunnerModel {
	return libQuery.QueryRunnerModel{DB: m.RawDB}
}

// SimpleTestData represents simplified test data for libQuery tests
type SimpleTestData struct {
	ID     int     `db:"id"`
	Name   string  `db:"name"`
	Value  string  `db:"value"`
	Active bool    `db:"active"`
	Count  int64   `db:"count"`
	Price  float64 `db:"price"`
}

func (s SimpleTestData) GetID() string { return "" }
func (s SimpleTestData) GetValue() any { return "" }

// MockQueryExpectations provides common query expectations for testing
type MockQueryExpectations struct {
	helper *MockDBHelper
}

// NewMockQueryExpectations creates a new mock query expectations helper
func NewMockQueryExpectations(helper *MockDBHelper) *MockQueryExpectations {
	return &MockQueryExpectations{helper: helper}
}

// SetupSelectQuery sets up a mock SELECT query expectation
func (m *MockQueryExpectations) SetupSelectQuery(tableName string, columns []string, values ...driver.Value) {
	sqlRegex := "SELECT \\* FROM " + tableName
	m.helper.ExpectQuery(sqlRegex, columns, values...)
}

// SetupInsertQuery sets up a mock INSERT query expectation
func (m *MockQueryExpectations) SetupInsertQuery(tableName string) {
	sqlRegex := "INSERT INTO " + tableName
	m.helper.ExpectExec(sqlRegex)
}

// SetupUpdateQuery sets up a mock UPDATE query expectation
func (m *MockQueryExpectations) SetupUpdateQuery(tableName string) {
	sqlRegex := "UPDATE " + tableName
	m.helper.ExpectExec(sqlRegex)
}

// SetupDeleteQuery sets up a mock DELETE query expectation
func (m *MockQueryExpectations) SetupDeleteQuery(tableName string) {
	sqlRegex := "DELETE FROM " + tableName
	m.helper.ExpectExec(sqlRegex)
}

// SetupSetConfigQuery sets up a mock set_config query expectation
func (m *MockQueryExpectations) SetupSetConfigQuery() {
	// The actual SQL pattern used by the application
	sqlRegex := "SELECT set_config\\$\\$1,\\$2, true\\$;"
	m.helper.ExpectExec(sqlRegex)
}

// SetupTransaction sets up a complete transaction expectation
func (m *MockQueryExpectations) SetupTransaction(queries ...func()) {
	m.helper.ExpectBegin()
	for _, query := range queries {
		query()
	}
	m.helper.ExpectCommit()
}

// CommonTestData provides common test data for libQuery tests
var CommonTestData = struct {
	SimpleColumns []string
	SimpleValues  []driver.Value
	SampleData    SimpleTestData
}{
	SimpleColumns: []string{"id", "name", "value", "active", "count", "price"},
	SimpleValues:  []driver.Value{1, "test", "value", true, int64(10), 12.5},
	SampleData: SimpleTestData{
		ID:     1,
		Name:   "test",
		Value:  "value",
		Active: true,
		Count:  10,
		Price:  12.5,
	},
}
