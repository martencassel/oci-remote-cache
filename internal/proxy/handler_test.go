package proxy

// import (
// 	"net/http"
// 	"net/http/httptest"
// 	"oci-remote-cache/internal/config"
// 	"testing"

// 	assert "github.com/stretchr/testify/assert"
// )

// func TestPing(t *testing.T) {
// 	req, _ := http.NewRequest("GET", "/v2/", nil)
// 	rr := httptest.NewRecorder()
// 	configProvider := config.NewRepoConfigInMemory()
// 	handler := NewHandler(&http.Client{}, configProvider)
// 	handler.ServeHTTP(rr, req)
// 	assert.Equal(t, http.StatusOK, rr.Code)
// 	assert.Equal(t, "registry/2.0", rr.Header().Get("Docker-Distribution-API-Version"))
// }

// func TestInvalidPath(t *testing.T) {
// 	req, _ := http.NewRequest("GET", "/invalid/path", nil)
// 	rr := httptest.NewRecorder()
// 	configProvider := config.NewRepoConfigInMemory()
// 	handler := NewHandler(&http.Client{}, configProvider)
// 	handler.ServeHTTP(rr, req)
// 	assert.Equal(t, http.StatusNotFound, rr.Code)
// }

// func TestUnknownRepoKey(t *testing.T) {
// 	req, _ := http.NewRequest("GET", "/v2/repo/manifests/latest", nil)
// 	rr := httptest.NewRecorder()
// 	configProvider := config.NewRepoConfigInMemory() // No repos added
// 	handler := NewHandler(&http.Client{}, configProvider)
// 	handler.ServeHTTP(rr, req)
// 	assert.Equal(t, http.StatusNotFound, rr.Code)
// }
