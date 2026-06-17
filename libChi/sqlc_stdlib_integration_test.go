package libChi_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/hmmftg/requestCore/libChi"
)

type sqlcStyleQueries struct {
	db *sql.DB
}

func (q sqlcStyleQueries) GetUserName(ctx context.Context, id string) (string, error) {
	row := q.db.QueryRowContext(ctx, "SELECT name FROM users WHERE id = $1", id)
	var name string
	if err := row.Scan(&name); err != nil {
		return "", err
	}
	return name, nil
}

func TestChiStdlibSqlcStyleWithSQLDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed creating sqlmock: %v", err)
	}
	defer db.Close()

	queries := sqlcStyleQueries{db: db}

	mock.ExpectQuery(regexp.QuoteMeta("SELECT name FROM users WHERE id = $1")).
		WithArgs("42").
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("alice"))

	router := chi.NewRouter()
	router.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		parser := libChi.InitParser(r, w)
		id := parser.GetUrlParam("id")
		name, err := queries.GetUserName(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := parser.SendJSONRespBody(http.StatusOK, map[string]string{"id": id, "name": name}); err != nil {
			t.Fatalf("failed writing json response: %v", err)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed parsing body: %v", err)
	}
	if resp["id"] != "42" || resp["name"] != "alice" {
		t.Fatalf("unexpected body: %+v", resp)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet db expectations: %v", err)
	}
}
