package server

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"mediaserver/internal/database"
	"mediaserver/internal/files"
)

type Server struct {
	port int

	db database.Service
}

func NewServer() *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	NewServer := &Server{
		port: port,

		db: database.New(),
	}
	go func() {
		files.PurgeExpired(NewServer.db, os.Getenv("STORAGE_PATH"))
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			files.PurgeExpired(NewServer.db, os.Getenv("STORAGE_PATH"))
		}
	}()

	files.ResumePendingJobs(NewServer.db, os.Getenv("STORAGE_PATH"))

	// Declare Server config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", NewServer.port),
		Handler:      NewServer.RegisterRoutes(),
		IdleTimeout:  2 * time.Minute,
		ReadTimeout:  30 * time.Minute,
		WriteTimeout: 30 * time.Minute,
	}

	return server
}
