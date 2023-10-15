package rest

import (
	"lightning/internal/runner"
	"lightning/internal/utils"
	"log"
	"net/http"
)

func (h handler) RunHandler(w http.ResponseWriter, r *http.Request) {
	var payload runner.RunPayload
	err := utils.DecodeJSON(r, &payload)
	if err != nil {
		utils.AnswerWithJSON(w, map[string]string{"message": err.Error()}, http.StatusBadRequest)
		return
	}

	log.Println("Running script...")
	result, err := h.runner.Run(payload)
	if err != nil {
		utils.AnswerWithJSON(w, map[string]string{"message": err.Error()}, http.StatusInternalServerError)
		return
	}

	utils.AnswerWithJSON(w, map[string]string{"result": result}, http.StatusOK)

}
