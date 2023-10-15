package rest

import (
	"lightning/internal/utils"
	"log"
	"net/http"
)

func StartAPIServer() error {
	handler := newHandler()
	log.Println("Listening on port 8080...")
	err := http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/run" && r.Method == "POST" {
			handler.RunHandler(w, r)
		} else {
			utils.AnswerWithJSON(w, map[string]string{"message": "Not found"}, http.StatusNotFound)
		}
	}))

	if err != nil {
		log.Println(err)
		return err
	}

	return nil

}
