package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"mediaserver/cmd/web"
	"mediaserver/internal/middleware"
	"mediaserver/internal/utils"

	"github.com/a-h/templ"
	"github.com/google/uuid"
)

func (s *Server) HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") != "" {
		templ.Handler(web.HomeContent()).ServeHTTP(w, r)
		return
	}
	templ.Handler(web.Home()).ServeHTTP(w, r)
}

func (s *Server) HelloWorldHandler(w http.ResponseWriter, r *http.Request) {
	resp := make(map[string]string)
	resp["message"] = "Hello World"

	jsonResp, err := json.Marshal(resp)
	if err != nil {
		slog.Error("error handling JSON marshal", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(jsonResp)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	jsonResp, _ := json.Marshal(s.db.Health())
	_, _ = w.Write(jsonResp)
}

func (s *Server) MeHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := s.db.GetUserByID(userID)
	if err != nil {
		utils.RespondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, user)
}
