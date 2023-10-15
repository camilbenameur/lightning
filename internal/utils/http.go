package utils

import (
	"encoding/json"
	"errors"
	"net/http"
)

func AnswerWithJSON(w http.ResponseWriter, payload any, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func DecodeJSON(r *http.Request, data any) error {
	if r.Header.Get("Content-Type") != "application/json" {
		return errors.New("Content-Type is not application/json")
	}

	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()

	if err := decoder.Decode(&data); err != nil {
		return err
	}

	return nil
}
