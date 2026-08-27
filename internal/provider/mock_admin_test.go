package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
)

// mockAdmin is an in-memory implementation of the subset of the Cloudinary
// Admin API used by this provider: upload presets and triggers.
type mockAdmin struct {
	mu       sync.Mutex
	seq      int
	presets  map[string]*mockPreset
	triggers map[string]map[string]any
}

type mockPreset struct {
	Name     string
	Unsigned bool
	Settings map[string]any
}

func newMockAdmin() *mockAdmin {
	return &mockAdmin{presets: map[string]*mockPreset{}, triggers: map[string]map[string]any{}}
}

func (m *mockAdmin) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(m.handle))
}

func (m *mockAdmin) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch {
	case strings.Contains(r.URL.Path, "/upload_presets"):
		m.handlePresets(w, r, remainder(r.URL.Path, "/upload_presets"))
	case strings.Contains(r.URL.Path, "/triggers"):
		m.handleTriggers(w, r, remainder(r.URL.Path, "/triggers"))
	default:
		http.NotFound(w, r)
	}
}

func remainder(path, endpoint string) string {
	idx := strings.Index(path, endpoint)
	return strings.Trim(strings.TrimPrefix(path[idx:], endpoint), "/")
}

func (m *mockAdmin) nextID(prefix string) string {
	m.seq++
	return prefix + strconv.Itoa(m.seq)
}

func (m *mockAdmin) handlePresets(w http.ResponseWriter, r *http.Request, name string) {
	switch {
	case name == "" && r.Method == http.MethodPost:
		m.createPreset(w, r)
	case name == "" && r.Method == http.MethodGet:
		m.listPresets(w)
	case name != "" && r.Method == http.MethodGet:
		m.getPreset(w, name)
	case name != "" && r.Method == http.MethodPut:
		m.updatePreset(w, r, name)
	case name != "" && r.Method == http.MethodDelete:
		m.deletePreset(w, name)
	default:
		http.NotFound(w, r)
	}
}

// splitPresetBody separates the preset-level fields from the upload parameters,
// which the Admin API returns nested under "settings".
func splitPresetBody(r *http.Request) (name string, unsigned bool, settings map[string]any) {
	body := map[string]any{}
	_ = json.NewDecoder(r.Body).Decode(&body)

	settings = map[string]any{}
	for key, value := range body {
		switch key {
		case "name":
			name, _ = value.(string)
		case "unsigned":
			unsigned, _ = value.(bool)
		default:
			settings[key] = value
		}
	}
	return name, unsigned, normalizeSettings(settings)
}

// The SDK sends list and map parameters joined into a single string, but the
// Admin API stores and returns them structured. The mock mirrors that.
func normalizeSettings(settings map[string]any) map[string]any {
	for _, key := range []string{"tags", "allowed_formats"} {
		if joined, ok := settings[key].(string); ok {
			settings[key] = strings.Split(joined, ",")
		}
	}
	for _, key := range []string{"context", "metadata"} {
		joined, ok := settings[key].(string)
		if !ok {
			continue
		}
		out := map[string]any{}
		for _, pair := range strings.Split(joined, "|") {
			k, v, found := strings.Cut(pair, "=")
			if found {
				out[k] = v
			}
		}
		settings[key] = out
	}
	return settings
}

func (m *mockAdmin) createPreset(w http.ResponseWriter, r *http.Request) {
	name, unsigned, settings := splitPresetBody(r)
	if name == "" {
		name = m.nextID("preset")
	}
	m.presets[name] = &mockPreset{Name: name, Unsigned: unsigned, Settings: settings}
	writeJSON(w, http.StatusOK, map[string]any{"message": "created", "name": name})
}

func (m *mockAdmin) listPresets(w http.ResponseWriter) {
	out := []any{}
	for _, p := range m.presets {
		out = append(out, map[string]any{"name": p.Name, "unsigned": p.Unsigned, "settings": p.Settings})
	}
	writeJSON(w, http.StatusOK, map[string]any{"presets": out})
}

func (m *mockAdmin) getPreset(w http.ResponseWriter, name string) {
	p, ok := m.presets[name]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "Upload preset not found"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": p.Name, "unsigned": p.Unsigned, "settings": p.Settings})
}

func (m *mockAdmin) updatePreset(w http.ResponseWriter, r *http.Request, name string) {
	if _, ok := m.presets[name]; !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "Upload preset not found"}})
		return
	}
	_, unsigned, settings := splitPresetBody(r)
	m.presets[name] = &mockPreset{Name: name, Unsigned: unsigned, Settings: settings}
	writeJSON(w, http.StatusOK, map[string]any{"message": "updated"})
}

func (m *mockAdmin) deletePreset(w http.ResponseWriter, name string) {
	if _, ok := m.presets[name]; !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "Upload preset not found"}})
		return
	}
	delete(m.presets, name)
	writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
}

func (m *mockAdmin) handleTriggers(w http.ResponseWriter, r *http.Request, id string) {
	switch {
	case id == "" && r.Method == http.MethodPost:
		m.createTrigger(w, r)
	case id == "" && r.Method == http.MethodGet:
		m.listTriggers(w)
	case id != "" && r.Method == http.MethodPut:
		m.updateTrigger(w, r, id)
	case id != "" && r.Method == http.MethodDelete:
		m.deleteTrigger(w, id)
	default:
		http.NotFound(w, r)
	}
}

func (m *mockAdmin) createTrigger(w http.ResponseWriter, r *http.Request) {
	body := map[string]any{}
	_ = json.NewDecoder(r.Body).Decode(&body)

	id := m.nextID("trigger")
	body["id"] = id
	body["uri_type"] = "http"
	body["created_at"] = "2026-08-19T10:00:00Z"
	body["updated_at"] = "2026-08-19T10:00:00Z"
	m.triggers[id] = body
	writeJSON(w, http.StatusOK, body)
}

func (m *mockAdmin) listTriggers(w http.ResponseWriter) {
	out := []any{}
	for _, t := range m.triggers {
		out = append(out, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"triggers": out})
}

func (m *mockAdmin) updateTrigger(w http.ResponseWriter, r *http.Request, id string) {
	existing, ok := m.triggers[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "Trigger not found"}})
		return
	}
	body := map[string]any{}
	_ = json.NewDecoder(r.Body).Decode(&body)
	for key, value := range body {
		existing[key] = value
	}
	existing["id"] = id
	existing["updated_at"] = "2026-08-19T11:00:00Z"
	// The Admin API does not return the trigger on update, so neither does the
	// mock: mapping the response straight back would blank the computed
	// attributes and Terraform would reject the apply as inconsistent.
	writeJSON(w, http.StatusOK, map[string]any{"message": "updated"})
}

func (m *mockAdmin) deleteTrigger(w http.ResponseWriter, id string) {
	if _, ok := m.triggers[id]; !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "Trigger not found"}})
		return
	}
	delete(m.triggers, id)
	writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
}
