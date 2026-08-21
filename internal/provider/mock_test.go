package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"
)

// mockProvisioning is an in-memory implementation of the subset of the
// Cloudinary Provisioning API used by this provider. It is used by both the
// integration test (driving the SDK directly) and the acceptance tests.
type mockProvisioning struct {
	mu   sync.Mutex
	seq  int
	envs map[string]*mockEnv
}

type mockEnv struct {
	ID        string
	Name      string
	CloudName string
	Enabled   bool
	CreatedAt time.Time
	// Auto-provisioned key returned at creation (api_access_keys).
	BootKey    string
	BootSecret string
	// Generated access keys, indexed by api_key.
	Keys map[string]*mockKey
}

type mockKey struct {
	Name      string
	APIKey    string
	APISecret string
	Enabled   bool
	CreatedAt time.Time
}

func newMockProvisioning() *mockProvisioning {
	return &mockProvisioning{envs: map[string]*mockEnv{}}
}

func (m *mockProvisioning) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(m.handle))
}

func (m *mockProvisioning) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Work with the path after ".../sub_accounts".
	idx := strings.Index(r.URL.Path, "/sub_accounts")
	if idx < 0 {
		http.NotFound(w, r)
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path[idx:], "/sub_accounts"), "/")
	segments := []string{}
	if rest != "" {
		segments = strings.Split(rest, "/")
	}

	switch {
	case len(segments) == 0 && r.Method == http.MethodPost:
		m.createEnv(w, r)
	case len(segments) == 0 && r.Method == http.MethodGet:
		m.listEnvs(w)
	case len(segments) == 1 && r.Method == http.MethodGet:
		m.getEnv(w, segments[0])
	case len(segments) == 1 && r.Method == http.MethodPut:
		m.updateEnv(w, r, segments[0])
	case len(segments) == 1 && r.Method == http.MethodDelete:
		m.deleteEnv(w, segments[0])
	case len(segments) == 2 && segments[1] == "access_keys" && r.Method == http.MethodPost:
		m.generateKey(w, r, segments[0])
	case len(segments) == 2 && segments[1] == "access_keys" && r.Method == http.MethodGet:
		m.listKeys(w, segments[0])
	case len(segments) == 3 && segments[1] == "access_keys" && r.Method == http.MethodPut:
		m.updateKey(w, r, segments[0], segments[2])
	case len(segments) == 3 && segments[1] == "access_keys" && r.Method == http.MethodDelete:
		m.deleteKey(w, segments[0], segments[2])
	default:
		http.NotFound(w, r)
	}
}

func (m *mockProvisioning) nextID(prefix string) string {
	m.seq++
	return prefix + strconv.Itoa(m.seq)
}

func (m *mockProvisioning) createEnv(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string  `json:"name"`
		CloudName *string `json:"cloud_name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	id := m.nextID("env")
	cloud := id + "-cloud"
	if body.CloudName != nil && *body.CloudName != "" {
		cloud = *body.CloudName
	}
	e := &mockEnv{
		ID:         id,
		Name:       body.Name,
		CloudName:  cloud,
		Enabled:    true,
		CreatedAt:  time.Unix(1700000000, 0).UTC(),
		BootKey:    m.nextID("bootkey"),
		BootSecret: m.nextID("bootsecret"),
		Keys:       map[string]*mockKey{},
	}
	e.Keys[e.BootKey] = &mockKey{
		Name:      "Root",
		APIKey:    e.BootKey,
		APISecret: e.BootSecret,
		Enabled:   true,
		CreatedAt: e.CreatedAt,
	}
	m.envs[id] = e
	writeJSON(w, http.StatusOK, m.envJSON(e, true))
}

func (m *mockProvisioning) listEnvs(w http.ResponseWriter) {
	out := []any{}
	for _, e := range m.envs {
		out = append(out, m.envJSON(e, false))
	}
	writeJSON(w, http.StatusOK, map[string]any{"sub_accounts": out})
}

func (m *mockProvisioning) getEnv(w http.ResponseWriter, id string) {
	e, ok := m.envs[id]
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, m.envJSON(e, false))
}

func (m *mockProvisioning) updateEnv(w http.ResponseWriter, r *http.Request, id string) {
	e, ok := m.envs[id]
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	var body struct {
		Name      *string `json:"name"`
		CloudName *string `json:"cloud_name"`
		Enabled   *bool   `json:"enabled"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Name != nil {
		e.Name = *body.Name
	}
	if body.CloudName != nil {
		e.CloudName = *body.CloudName
	}
	if body.Enabled != nil {
		e.Enabled = *body.Enabled
	}
	writeJSON(w, http.StatusOK, m.envJSON(e, false))
}

func (m *mockProvisioning) deleteEnv(w http.ResponseWriter, id string) {
	if _, ok := m.envs[id]; !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	delete(m.envs, id)
	writeJSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

func (m *mockProvisioning) generateKey(w http.ResponseWriter, r *http.Request, envID string) {
	e, ok := m.envs[envID]
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	var body struct {
		Name    *string `json:"name"`
		Enabled *bool   `json:"enabled"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	k := &mockKey{
		APIKey:    m.nextID("apikey"),
		APISecret: m.nextID("apisecret"),
		Enabled:   true,
		CreatedAt: time.Unix(1700000100, 0).UTC(),
	}
	if body.Name != nil {
		k.Name = *body.Name
	}
	if body.Enabled != nil {
		k.Enabled = *body.Enabled
	}
	e.Keys[k.APIKey] = k
	writeJSON(w, http.StatusOK, keyJSON(k, true))
}

func (m *mockProvisioning) listKeys(w http.ResponseWriter, envID string) {
	e, ok := m.envs[envID]
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	out := []any{}
	for _, k := range e.Keys {
		out = append(out, keyJSON(k, true)) // the list endpoint returns the secret
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_keys": out, "total": len(out)})
}

func (m *mockProvisioning) updateKey(w http.ResponseWriter, r *http.Request, envID, apiKey string) {
	e, ok := m.envs[envID]
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	k, ok := e.Keys[apiKey]
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	var body struct {
		Name    *string `json:"name"`
		Enabled *bool   `json:"enabled"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Name != nil {
		k.Name = *body.Name
	}
	if body.Enabled != nil {
		k.Enabled = *body.Enabled
	}
	writeJSON(w, http.StatusOK, keyJSON(k, false))
}

func (m *mockProvisioning) deleteKey(w http.ResponseWriter, envID, apiKey string) {
	e, ok := m.envs[envID]
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if _, ok := e.Keys[apiKey]; !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	delete(e.Keys, apiKey)
	writeJSON(w, http.StatusOK, map[string]any{"message": "ok"})
}

func (m *mockProvisioning) envJSON(e *mockEnv, withSecret bool) map[string]any {
	secret := ""
	if withSecret {
		secret = e.BootSecret
	}
	return map[string]any{
		"id":         e.ID,
		"name":       e.Name,
		"cloud_name": e.CloudName,
		"enabled":    e.Enabled,
		"created_at": e.CreatedAt.Format(time.RFC3339),
		"api_access_keys": []any{
			map[string]any{"key": e.BootKey, "secret": secret, "enabled": true},
		},
	}
}

func keyJSON(k *mockKey, withSecret bool) map[string]any {
	out := map[string]any{
		"name":       k.Name,
		"api_key":    k.APIKey,
		"enabled":    k.Enabled,
		"created_at": k.CreatedAt.Format(time.RFC3339),
	}
	if withSecret {
		out["api_secret"] = k.APISecret
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
