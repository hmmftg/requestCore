package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hmmftg/requestCore/libChi"
)

func main() {
	router := chi.NewRouter()

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		parser := libChi.InitParser(r, w)
		if err := parser.SendJSONRespBody(http.StatusOK, map[string]string{"status": "ok"}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	router.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		parser := libChi.InitParser(r, w)
		id := parser.GetUrlParam("id")
		if err := parser.SendJSONRespBody(http.StatusOK, map[string]string{"id": id}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	router.Post("/echo", func(w http.ResponseWriter, r *http.Request) {
		parser := libChi.InitParser(r, w)
		var body struct {
			Message string `json:"message"`
		}
		if err := parser.GetBody(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := parser.SendJSONRespBody(http.StatusOK, body); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	log.Println("chi-hello listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
