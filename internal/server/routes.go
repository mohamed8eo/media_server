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
	r.Handle("/assets/*", fileServer)

	// Guest-only UI pages (Redirects authenticated users away)
	r.Group(func(gr chi.Router) {
		gr.Use(middleware.GuestMiddleware(s.db))
		gr.Get("/sign-up", templ.Handler(web.SignUp()).ServeHTTP)
		gr.Get("/sign-in", templ.Handler(web.SignIn()).ServeHTTP)
	})

	// Public UI pages
	r.Get("/web", templ.Handler(web.HelloForm()).ServeHTTP)

	// Protected UI pages (Redirects unauthenticated users to /sign-in)
	r.Group(func(gr chi.Router) {
		gr.Use(middleware.UIAuthMiddleware(s.db))
		gr.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<h1>Welcome to MediaServer Dashboard</h1><a href="/api/auth/logout">Logout</a>`))
		})
	})
}

func (s *Server) RegisterAPIRoutes(r chi.Router) {
	r.Get("/", s.HelloWorldHandler)
	r.Get("/health", s.healthHandler)
	r.Mount("/auth", auth.NewRouter(s.db))
	r.Post("/hello", web.HelloWebHandler)

	// Protected API routes (Returns 401 Unauthorized for API clients)
	r.Group(func(gr chi.Router) {
		gr.Use(middleware.AuthMiddleware(s.db))
		gr.Mount("/file", files.NewRouter(s.db))
		gr.Get("/me", s.MeHandler)
	})
}
