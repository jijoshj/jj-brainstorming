package controllers

import (
	"encoding/json"
	"net/http"
)

type BaseController struct{}

func (bc *BaseController) SetCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")
}

func (bc *BaseController) RespondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	bc.SetCommonHeaders(w)
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (bc *BaseController) RespondError(w http.ResponseWriter, statusCode int, message string) {
	bc.RespondJSON(w, statusCode, map[string]string{"error": message})
}

func (bc *BaseController) HandlePreflight(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == "OPTIONS" {
		bc.SetCommonHeaders(w)
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}
