package server

import (
	"net/http"

	"mediaserver/cmd/web"
	"mediaserver/internal/auth"
	"mediaserver/internal/files"
	"mediaserver/internal/middleware"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := chi.NewRouter()

	// Global Middleware
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.CORS())
	r.Use(middleware.SecurityHeaders())

	// 1. UI & Frontend Routes
	s.RegisterUIRoutes(r)

	// 2. Backend API Routes
	r.Route("/api", func(apiRouter chi.Router) {
		s.RegisterAPIRoutes(apiRouter)
	})

	return r
}

func (s *Server) RegisterUIRoutes(r chi.Router) {
	// Static assets
	fileServer := http.FileServer(http.FS(web.Files))
	r.Handle("/assets/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		fileServer.ServeHTTP(w, r)
	}))

	// Guest-only UI pages (Redirects authenticated users away)
	r.Group(func(gr chi.Router) {
		gr.Use(middleware.GuestMiddleware(s.db))
		gr.Get("/sign-up", templ.Handler(web.SignUp()).ServeHTTP)
		gr.Get("/sign-in", templ.Handler(web.SignIn()).ServeHTTP)
		gr.Get("/forgot-password", templ.Handler(web.ForgotPassword()).ServeHTTP)
	})

	// Protected UI pages (Redirects unauthenticated users to /sign-in)
	r.Group(func(gr chi.Router) {
		gr.Use(middleware.UIAuthMiddleware(s.db))
		gr.Get("/", s.HomeHandler)
		gr.Get("/watch/{id}", s.WatchHandler)
		gr.Get("/upload", templ.Handler(web.Upload()).ServeHTTP)
		gr.Get("/settings", templ.Handler(web.Settings()).ServeHTTP)
	})
}

func (s *Server) RegisterAPIRoutes(r chi.Router) {
	r.Get("/", s.HelloWorldHandler)
	r.Get("/health", s.healthHandler)
	r.Mount("/auth", auth.NewRouter(s.db))

	// Protected API routes (Returns 401 Unauthorized for API clients)
	r.Group(func(gr chi.Router) {
		gr.Use(middleware.AuthMiddleware(s.db))
		gr.Mount("/file", files.NewRouter(s.db))
		gr.Get("/me", s.MeHandler)
	})
}
