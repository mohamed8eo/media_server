package server

import (
	"net/http"

	"mediaserver/internal/utils"
)

// APIHandler defines a handler function that returns an error
type APIHandler func(w http.ResponseWriter, r *http.Request) error

// MakeHandler wraps an APIHandler to automatically handle returned errors
func MakeHandler(h APIHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			utils.RespondWithError(w, http.StatusInternalServerError, err.Error())
		}
	}
}
