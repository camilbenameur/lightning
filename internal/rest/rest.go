package rest

import (
	"lightning/internal/utils"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func StartAPIServer() error {
	r := mux.NewRouter()

	h := newHandler()

	corsMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("Request Method: %s", r.Method)
			log.Printf("Request Headers: %+v", r.Header)

			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")

			next.ServeHTTP(w, r)
		})
	}

	r.Use(corsMiddleware)

	// OPTIONS handler for /run
	r.HandleFunc("/run", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
	}).Methods("OPTIONS")

	// POST handler for /run
	r.HandleFunc("/run", func(w http.ResponseWriter, r *http.Request) {
		h.RunHandler(w, r)
	}).Methods("POST")

	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		utils.AnswerWithJSON(w, map[string]string{"message": "Not found"}, http.StatusNotFound)
	})

	srv := &http.Server{
		Handler: r,
		Addr:    ":8080",
	}

	log.Println("Listening on port 8080...")

	err := srv.ListenAndServe()
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}
