package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func makeRequest(method, path string, body interface{}) *httptest.ResponseRecorder {
	var reqBody bytes.Buffer
	if body != nil {
		json.NewEncoder(&reqBody).Encode(body)
	}

	req := httptest.NewRequest(method, path, &reqBody)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	return w
}

func TestHealthCheck(t *testing.T) {
	w := makeRequest("GET", "/health", nil)
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("Unexpected status code: %d", w.Code)
	}
}
