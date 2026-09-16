package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"mediaserver/cmd/web"
	"mediaserver/internal/middleware"
	"mediaserver/internal/utils"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") != "" {
		templ.Handler(web.HomeContent()).ServeHTTP(w, r)
		return
	}
	templ.Handler(web.Home()).ServeHTTP(w, r)
}

// WatchHandler renders a dedicated player for a video owned by the current user.
// The media bytes continue to be delivered by the existing authenticated file endpoint.
func (s *Server) WatchHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		templ.Handler(web.WatchUnavailable()).ServeHTTP(w, r)
		return
	}

	fileID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		templ.Handler(web.WatchUnavailable()).ServeHTTP(w, r)
		return
	}

	file, err := s.db.GetFileByID(fileID)
	if err != nil || file.UserID != userID || !strings.HasPrefix(strings.ToLower(file.MimeType), "video/") {
		w.WriteHeader(http.StatusNotFound)
		templ.Handler(web.WatchUnavailable()).ServeHTTP(w, r)
		return
	}

	templ.Handler(web.Watch(*file)).ServeHTTP(w, r)
}

// AudioHandler renders a dedicated audio player for an audio file owned by the current user.
func (s *Server) AudioHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		templ.Handler(web.AudioUnavailable()).ServeHTTP(w, r)
		return
	}

	fileID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		templ.Handler(web.AudioUnavailable()).ServeHTTP(w, r)
		return
	}

	file, err := s.db.GetFileByID(fileID)
	mimeLower := strings.ToLower(file.MimeType)
	if err != nil || file.UserID != userID || (!strings.HasPrefix(mimeLower, "audio/") && !strings.HasPrefix(mimeLower, "video/")) {
		w.WriteHeader(http.StatusNotFound)
		templ.Handler(web.AudioUnavailable()).ServeHTTP(w, r)
		return
	}

	templ.Handler(web.AudioPlayer(*file)).ServeHTTP(w, r)
}

// ReaderHandler renders a dedicated reader interface for a document owned by the current user.
func (s *Server) ReaderHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		templ.Handler(web.ReaderUnavailable()).ServeHTTP(w, r)
		return
	}

	fileID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		templ.Handler(web.ReaderUnavailable()).ServeHTTP(w, r)
		return
	}

	file, err := s.db.GetFileByID(fileID)
	if err != nil || file.UserID != userID {
		w.WriteHeader(http.StatusNotFound)
		templ.Handler(web.ReaderUnavailable()).ServeHTTP(w, r)
		return
	}

	mimeLower := strings.ToLower(file.MimeType)
	isDoc := strings.Contains(mimeLower, "pdf") ||
		strings.Contains(mimeLower, "epub") ||
		strings.Contains(mimeLower, "document") ||
		strings.Contains(mimeLower, "word") ||
		strings.Contains(mimeLower, "sheet") ||
		strings.HasPrefix(mimeLower, "text/")

	if !isDoc {
		w.WriteHeader(http.StatusNotFound)
		templ.Handler(web.ReaderUnavailable()).ServeHTTP(w, r)
		return
	}

	templ.Handler(web.Reader(*file)).ServeHTTP(w, r)
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
