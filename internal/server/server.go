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

func NewServer(dbArgs ...database.Service) *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	var db database.Service
	if len(dbArgs) > 0 && dbArgs[0] != nil {
		db = dbArgs[0]
	} else {
		db = database.New()
	}
	NewServer := &Server{
		port: port,

		db: db,
	}
	go func() {
		files.PurgeExpired(NewServer.db, os.Getenv("STORAGE_PATH"))
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			files.PurgeExpired(NewServer.db, os.Getenv("STORAGE_PATH"))
		}
	}()

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
