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

func TestBatchOperationsAndTrash(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("APP_ENV", "local")
	os.Setenv("BLUEPRINT_DB_URL", "file:files_test_batch?mode=memory&cache=shared")
	os.Setenv("STORAGE_PATH", t.TempDir())
	database.Reset()

	db := database.New()
	userID, err := db.CreateUser("testbatch@example.com", "passwordhash")
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

	// 1. Upload a file
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	fileWriter, _ := bodyWriter.CreateFormFile("file", "batchfile.txt")
	fileWriter.Write([]byte("batch test content"))
	bodyWriter.Close()

	req, _ := http.NewRequest("POST", server.URL+"/", bodyBuf)
	req.Header.Set("Content-Type", bodyWriter.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload failed: %v, status: %v", err, resp.StatusCode)
	}
	resp.Body.Close()

	// Get file ID from list
	req, _ = http.NewRequest("GET", server.URL+"/", nil)
	resp, _ = http.DefaultClient.Do(req)
	var listResp ListResponse
	json.NewDecoder(resp.Body).Decode(&listResp)
	resp.Body.Close()

	if len(listResp.Files) != 1 {
		t.Fatalf("expected 1 file")
	}
	fileID := listResp.Files[0].ID

	// 2. Batch Delete (Soft delete)
	batchDelBody, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"kind": "file", "id": fileID},
		},
	})
	req, _ = http.NewRequest("POST", server.URL+"/batch/delete", bytes.NewReader(batchDelBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("batch delete failed: %v, status: %v", err, resp.StatusCode)
	}
	resp.Body.Close()

	// 3. Verify in Trash
	req, _ = http.NewRequest("GET", server.URL+"/trash", nil)
	resp, _ = http.DefaultClient.Do(req)
	var trashResp struct {
		Items []struct {
			Kind string `json:"kind"`
			ID   string `json:"id"`
		} `json:"items"`
	}
	json.NewDecoder(resp.Body).Decode(&trashResp)
	resp.Body.Close()
	if len(trashResp.Items) != 1 {
		t.Fatalf("expected 1 item in trash, got %d", len(trashResp.Items))
	}

	// 4. Restore from Trash
	restoreBody, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"kind": "file", "id": fileID},
		},
	})
	req, _ = http.NewRequest("POST", server.URL+"/trash/restore", bytes.NewReader(restoreBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("restore failed: %v, status: %v", err, resp.StatusCode)
	}
	resp.Body.Close()

	// 5. Verify batch download
	dlBody, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"kind": "file", "id": fileID},
		},
	})
	req, _ = http.NewRequest("POST", server.URL+"/batch/download", bytes.NewReader(dlBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("batch download failed: %v, status: %v", err, resp.StatusCode)
	}
	resp.Body.Close()
}

func TestDownloadURLHandler(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("APP_ENV", "local")
	os.Setenv("BLUEPRINT_DB_URL", "file:files_test_dlurl?mode=memory&cache=shared")
	os.Setenv("STORAGE_PATH", t.TempDir())
	database.Reset()

	db := database.New()
	userID, err := db.CreateUser("testdlurl@example.com", "passwordhash")
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

	payload, _ := json.Marshal(DownloadURLRequest{
		URL:     "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		Quality: "1080p",
		Folder:  "/",
	})

	req, err := http.NewRequest("POST", server.URL+"/download-url", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("expected status 202 Accepted, got %v", resp.Status)
	}

	var resMap map[string]string
	json.NewDecoder(resp.Body).Decode(&resMap)
	jobID := resMap["job_id"]
	if jobID == "" {
		t.Fatalf("expected job_id in response")
	}

	// Test GET /jobs/{id}
	reqGet, _ := http.NewRequest("GET", server.URL+"/jobs/"+jobID, nil)
	respGet, err := http.DefaultClient.Do(reqGet)
	if err != nil || respGet.StatusCode != http.StatusOK {
		t.Fatalf("get job failed: %v, status: %v", err, respGet.StatusCode)
	}
	defer respGet.Body.Close()

	var jobResp map[string]any
	json.NewDecoder(respGet.Body).Decode(&jobResp)
	if jobResp["id"] != jobID {
		t.Errorf("expected job id %s, got %v", jobID, jobResp["id"])
	}
}
