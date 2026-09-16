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

// TLSFiles returns the cert/key paths configured for local HTTPS via
// TLS_CERT_FILE / TLS_KEY_FILE env vars. ok is false when either is unset,
// meaning the caller should fall back to plain HTTP.
func TLSFiles() (certFile, keyFile string, ok bool) {
	certFile = os.Getenv("TLS_CERT_FILE")
	keyFile = os.Getenv("TLS_KEY_FILE")
	return certFile, keyFile, certFile != "" && keyFile != ""
}
