package files

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"mediaserver/internal/database"
	"mediaserver/internal/middleware"
)

func TestUploadAndListSubfolder(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("APP_ENV", "local")
	os.Setenv("BLUEPRINT_DB_URL", "file:files_test?mode=memory&cache=shared")
	os.Setenv("STORAGE_PATH", t.TempDir())
	database.Reset()

	db := database.New()
	userID, err := db.CreateUser("test@example.com", "passwordhash")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	router := NewRouter(db)

	// Wrap with mock user context middleware
	authHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
		router.ServeHTTP(w, r.WithContext(ctx))
	})

	server := httptest.NewServer(authHandler)
	defer server.Close()

	// 1. Create a subfolder via POST /mkdir
	mkdirBody, _ := json.Marshal(map[string]string{"path": "/testing"})
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
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201 Created for mkdir, got %v", resp.Status)
	}

	// 2. Upload a file into /testing
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	bodyWriter.WriteField("folder", "/testing")
	fileWriter, err := bodyWriter.CreateFormFile("file", "testfile.txt")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	fileWriter.Write([]byte("hello world"))
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
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201 Created for upload, got %v", resp.Status)
	}

	// 3. List files via GET /
	req, err = http.NewRequest("GET", server.URL+"/", nil)
	if err != nil {
		t.Fatalf("failed to create list request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK for list, got %v", resp.Status)
	}

	var listResp ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}

	if len(listResp.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(listResp.Files))
	}

	if listResp.Files[0].Folder != "/testing" {
		t.Errorf("expected file folder to be '/testing', got '%s'", listResp.Files[0].Folder)
	}
}
