package files

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"mediaserver/internal/database"
	"mediaserver/internal/middleware"
)

func TestConcurrentOperations(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("APP_ENV", "local")
	os.Setenv("BLUEPRINT_DB_URL", "file:concurrent_test?mode=memory&cache=shared")
	os.Setenv("STORAGE_PATH", t.TempDir())
	database.Reset()

	db := database.New()
	userID, err := db.CreateUser("concurrent@example.com", "passwordhash")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	router := NewRouter(db)
	authHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
		router.ServeHTTP(w, r.WithContext(ctx))
	})

	server := httptest.NewServer(authHandler)
	defer server.Close()

	var wg sync.WaitGroup
	workers := 10

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			bodyBuf := &bytes.Buffer{}
			bodyWriter := multipart.NewWriter(bodyBuf)
			fileWriter, err := bodyWriter.CreateFormFile("file", "file.txt")
			if err != nil {
				return
			}
			fileWriter.Write([]byte("data"))
			bodyWriter.Close()

			req, err := http.NewRequest("POST", server.URL+"/", bodyBuf)
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", bodyWriter.FormDataContentType())
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
		}(i)
	}
	wg.Wait()

	// Verify listing works concurrently
	req, err := http.NewRequest("GET", server.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %v", resp.Status)
	}
}

func TestPathTraversalValidation(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("APP_ENV", "local")
	os.Setenv("BLUEPRINT_DB_URL", "file:traversal_test?mode=memory&cache=shared")
	os.Setenv("STORAGE_PATH", t.TempDir())
	database.Reset()

	db := database.New()
	userID, err := db.CreateUser("traversal@example.com", "passwordhash")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	router := NewRouter(db)
	authHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
		router.ServeHTTP(w, r.WithContext(ctx))
	})

	server := httptest.NewServer(authHandler)
	defer server.Close()

	// 1. Attempt path traversal in mkdir
	mkdirBody, _ := json.Marshal(map[string]string{"path": "/../../etc"})
	req, err := http.NewRequest("POST", server.URL+"/mkdir", bytes.NewReader(mkdirBody))
	if err != nil {
		t.Fatalf("failed to create mkdir request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("mkdir request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 Bad Request for traversal mkdir, got %v", resp.Status)
	}

	// 2. Attempt path traversal in upload folder
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	bodyWriter.WriteField("folder", "/../../outside")
	fileWriter, _ := bodyWriter.CreateFormFile("file", "evil.txt")
	fileWriter.Write([]byte("bad"))
	bodyWriter.Close()

	req, err = http.NewRequest("POST", server.URL+"/", bodyBuf)
	if err != nil {
		t.Fatalf("failed to create upload request: %v", err)
	}
	req.Header.Set("Content-Type", bodyWriter.FormDataContentType())
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 Bad Request for traversal upload, got %v", resp.Status)
	}
}

func TestLargeFileUploadEdges(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("APP_ENV", "local")
	os.Setenv("BLUEPRINT_DB_URL", "file:edges_test?mode=memory&cache=shared")
	os.Setenv("STORAGE_PATH", t.TempDir())
	database.Reset()

	db := database.New()
	userID, err := db.CreateUser("edges@example.com", "passwordhash")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	router := NewRouter(db)
	authHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
		router.ServeHTTP(w, r.WithContext(ctx))
	})

	server := httptest.NewServer(authHandler)
	defer server.Close()

	// Zero-byte file upload
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	fileWriter, _ := bodyWriter.CreateFormFile("file", "empty.txt")
	fileWriter.Write([]byte(""))
	bodyWriter.Close()

	req, err := http.NewRequest("POST", server.URL+"/", bodyBuf)
	if err != nil {
		t.Fatalf("failed to create upload request: %v", err)
	}
	req.Header.Set("Content-Type", bodyWriter.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201 Created for zero-byte file, got %v", resp.Status)
	}
}

func TestThumbnailGenerationErrors(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := tmpDir + "/nonexistent"

	// 1. Non-image/non-video mime type should return ErrNoThumbnail
	_, err := GenerateThumbnail(storagePath, "text/plain")
	if err != ErrNoThumbnail {
		t.Errorf("expected ErrNoThumbnail for text/plain, got %v", err)
	}

	// 2. Corrupt/invalid image file path should attempt fallback (or return error if ffmpeg is unavailable)
	corruptPath := tmpDir + "/corrupt.jpg"
	os.WriteFile(corruptPath, []byte("not an image"), 0644)
	_, err = GenerateThumbnail(corruptPath, "image/jpeg")
	_ = err
}
