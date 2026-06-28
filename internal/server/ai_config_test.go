package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAIGlobalNeverExposesKey(t *testing.T) {
	h := newHarness(t)
	resp := h.do(t, http.MethodGet, "/api/ai/global", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("global: want 200, got %d", resp.Status)
	}
	body := string(resp.Body)
	if !strings.Contains(body, `"configured":true`) || !strings.Contains(body, `"model":"gpt-4o"`) {
		t.Fatalf("global status missing presence/model: %s", body)
	}
	if strings.Contains(body, "sk-global-secret") {
		t.Fatalf("global key leaked over API: %s", body)
	}
}

func TestNoGlobalWritePath(t *testing.T) {
	h := newHarness(t)
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		if resp := h.do(t, m, "/api/ai/global", "op", strings.NewReader(`{}`)); resp.Status < 400 {
			t.Fatalf("%s /api/ai/global should not succeed, got %d", m, resp.Status)
		}
	}
}

func TestUserProviderCRUDOverHTTP(t *testing.T) {
	h := newHarness(t)

	body := `{"kind":"openai","name":"Mine","apiKey":"sk-user-secret","models":["gpt-4o"],"model":"gpt-4o"}`
	resp := h.do(t, http.MethodPost, "/api/me/ai/config", "op", strings.NewReader(body))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create: want 201, got %d (%s)", resp.Status, resp.Body)
	}
	created := string(resp.Body)
	if strings.Contains(created, "sk-user-secret") {
		t.Fatalf("user key leaked in create response: %s", created)
	}
	if !strings.Contains(created, `"hasKey":true`) {
		t.Fatalf("create response should report hasKey: %s", created)
	}
	id := createConnID(t, resp) // reuses the {"id":...} extractor

	dup := `{"kind":"openai","name":"mine","apiKey":"sk-user-secret","models":["gpt-4o"],"model":"gpt-4o"}`
	if resp := h.do(t, http.MethodPost, "/api/me/ai/config", "op", strings.NewReader(dup)); resp.Status != http.StatusConflict {
		t.Fatalf("duplicate provider name: want 409, got %d (%s)", resp.Status, resp.Body)
	}

	resp = h.do(t, http.MethodGet, "/api/me/ai/config", "op", nil)
	if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), `"name":"Mine"`) {
		t.Fatalf("list: status=%d body=%s", resp.Status, resp.Body)
	}
	if strings.Contains(string(resp.Body), "sk-user-secret") {
		t.Fatalf("user key leaked in list: %s", resp.Body)
	}

	resp = h.do(t, http.MethodGet, "/api/me/ai/config/"+id+"/models", "op", nil)
	if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), "gpt-4o") {
		t.Fatalf("models: status=%d body=%s", resp.Status, resp.Body)
	}

	if resp := h.do(t, http.MethodDelete, "/api/me/ai/config/"+id, "viewer", nil); resp.Status != http.StatusNotFound {
		t.Fatalf("cross-owner delete: want 404, got %d", resp.Status)
	}

	if resp := h.do(t, http.MethodDelete, "/api/me/ai/config/"+id, "op", nil); resp.Status != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d", resp.Status)
	}
}

func TestConnectionAIModePersistsAndClearsDestructive(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/connections", "op", strings.NewReader(
		`{"name":"ai-rw","protocol":"tester","config":{"host":"h"},"aiMode":"read_write","aiAllowDestructive":true}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create: %d (%s)", resp.Status, resp.Body)
	}
	id := createConnID(t, resp)
	detail := h.do(t, http.MethodGet, "/api/connections/"+id, "op", nil)
	if !strings.Contains(string(detail.Body), `"aiMode":"read_write"`) ||
		!strings.Contains(string(detail.Body), `"aiAllowDestructive":true`) {
		t.Fatalf("read_write+destructive not persisted: %s", detail.Body)
	}
	list := h.do(t, http.MethodGet, "/api/connections", "op", nil)
	if !strings.Contains(string(list.Body), `"aiMode":"read_write"`) ||
		!strings.Contains(string(list.Body), `"aiAllowDestructive":true`) {
		t.Fatalf("connection list must expose ai mode for launcher gating: %s", list.Body)
	}

	resp = h.do(t, http.MethodPost, "/api/connections", "op", strings.NewReader(
		`{"name":"ai-ro","protocol":"tester","config":{"host":"h"},"aiMode":"read_only","aiAllowDestructive":true}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create read_only: %d (%s)", resp.Status, resp.Body)
	}
	roID := createConnID(t, resp)
	detail = h.do(t, http.MethodGet, "/api/connections/"+roID, "op", nil)
	if !strings.Contains(string(detail.Body), `"aiAllowDestructive":false`) {
		t.Fatalf("read_only must clear destructive opt-in: %s", detail.Body)
	}

	if resp := h.do(t, http.MethodPost, "/api/connections", "op", strings.NewReader(
		`{"name":"ai-bad","protocol":"tester","config":{"host":"h"},"aiMode":"bogus"}`)); resp.Status != http.StatusBadRequest {
		t.Fatalf("invalid ai mode: want 400, got %d", resp.Status)
	}
}

func TestUserProviderValidationReturns400(t *testing.T) {
	h := newHarness(t)
	resp := h.do(t, http.MethodPost, "/api/me/ai/config", "op", strings.NewReader(`{"kind":"bogus","name":"x","model":"m"}`))
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("bad kind: want 400, got %d (%s)", resp.Status, resp.Body)
	}
	resp = h.do(t, http.MethodPost, "/api/me/ai/config", "op", strings.NewReader(`{"kind":"openai","name":"x","apiKey":"sk-user-secret"}`))
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("missing model: want 400, got %d (%s)", resp.Status, resp.Body)
	}
	resp = h.do(t, http.MethodPost, "/api/me/ai/config", "op", strings.NewReader(`{"kind":"openai_compatible","name":"x","model":"m"}`))
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("missing compatible base URL: want 400, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestPreviewAIProviderModels(t *testing.T) {
	h := newHarness(t)
	models := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"llama3"}]}`))
	}))
	defer models.Close()

	resp := h.do(t, http.MethodPost, "/api/me/ai/models", "op", strings.NewReader(
		`{"kind":"openai_compatible","name":"Local","baseUrl":"`+models.URL+`","model":"llama3"}`))
	if resp.Status != http.StatusOK {
		t.Fatalf("preview models: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	body := string(resp.Body)
	if !strings.Contains(body, "llama3") {
		t.Fatalf("provider models missing: %s", body)
	}
	if strings.Contains(body, "sk-user-secret") {
		t.Fatalf("key leaked in model preview: %s", body)
	}

	anthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer anthropic.Close()

	resp = h.do(t, http.MethodPost, "/api/me/ai/models", "op", strings.NewReader(
		`{"kind":"anthropic","name":"Anthropic","baseUrl":"`+anthropic.URL+`","model":"claude-sonnet-4-5"}`))
	if resp.Status != http.StatusBadRequest || !strings.Contains(string(resp.Body), "provider returned HTTP 401 Unauthorized") {
		t.Fatalf("provider HTTP error: want clear 400, got %d (%s)", resp.Status, resp.Body)
	}
}

func TestDraftAIProviderTestValidation(t *testing.T) {
	h := newHarness(t)
	models := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"llama3"}]}`))
	}))
	defer models.Close()

	for name, body := range map[string]string{
		"missing model":    `{"kind":"openai","apiKey":"sk-user-secret"}`,
		"missing key":      `{"kind":"openai","model":"gpt-4o"}`,
		"missing base URL": `{"kind":"openai_compatible","model":"llama3"}`,
	} {
		resp := h.do(t, http.MethodPost, "/api/me/ai/test", "op", strings.NewReader(body))
		if resp.Status != http.StatusBadRequest {
			t.Fatalf("%s: want 400, got %d (%s)", name, resp.Status, resp.Body)
		}
	}
	resp := h.do(t, http.MethodPost, "/api/me/ai/test", "op", strings.NewReader(
		`{"kind":"openrouter","name":"OpenRouter","model":"z-ai/glm-4.5-air","models":["z-ai/glm-4.5-air"]}`))
	if resp.Status != http.StatusBadRequest || !strings.Contains(string(resp.Body), "api key is required") {
		t.Fatalf("missing openrouter key: want clear 400, got %d (%s)", resp.Status, resp.Body)
	}

	resp = h.do(t, http.MethodPost, "/api/me/ai/test", "op", strings.NewReader(
		`{"kind":"openai_compatible","baseUrl":"`+models.URL+`","model":"llama3"}`))
	if resp.Status != http.StatusOK || !strings.Contains(string(resp.Body), `"ok":true`) {
		t.Fatalf("valid draft test: status=%d body=%s", resp.Status, resp.Body)
	}
}

func TestAIConversationRoutesAreConnectionScoped(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/connections", "op", strings.NewReader(
		`{"name":"other-ai","protocol":"tester","config":{"host":"h"}}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create other connection: %d (%s)", resp.Status, resp.Body)
	}
	otherConnID := createConnID(t, resp)

	resp = h.do(t, http.MethodPost, "/api/connections/c-op/ai/conversations", "op", strings.NewReader(`{}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create conversation: %d (%s)", resp.Status, resp.Body)
	}
	convID := createConnID(t, resp)

	for name, tc := range map[string]struct {
		method string
		path   string
		body   string
	}{
		"get":      {method: http.MethodGet, path: "/api/connections/" + otherConnID + "/ai/conversations/" + convID},
		"messages": {method: http.MethodGet, path: "/api/connections/" + otherConnID + "/ai/conversations/" + convID + "/messages"},
		"rename":   {method: http.MethodPut, path: "/api/connections/" + otherConnID + "/ai/conversations/" + convID, body: `{"title":"wrong"}`},
		"delete":   {method: http.MethodDelete, path: "/api/connections/" + otherConnID + "/ai/conversations/" + convID},
	} {
		resp := h.do(t, tc.method, tc.path, "op", strings.NewReader(tc.body))
		if resp.Status != http.StatusNotFound {
			t.Fatalf("%s through wrong connection: want 404, got %d (%s)", name, resp.Status, resp.Body)
		}
	}

	resp = h.do(t, http.MethodGet, "/api/connections/c-op/ai/conversations/"+convID, "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("original conversation should remain accessible: %d (%s)", resp.Status, resp.Body)
	}
}

func TestAIConversationResponsesUseClientJSONShape(t *testing.T) {
	h := newHarness(t)

	resp := h.do(t, http.MethodPost, "/api/connections/c-op/ai/conversations", "op", strings.NewReader(`{}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create conversation: %d (%s)", resp.Status, resp.Body)
	}
	convID := createConnID(t, resp)
	body := string(resp.Body)
	for _, want := range []string{`"id":"`, `"connectionId":"c-op"`, `"title":"New conversation"`, `"titleResolved":false`, `"providerId":""`, `"model":"gpt-4o"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("conversation response missing %s: %s", want, body)
		}
	}
	for _, forbidden := range []string{`"ID"`, `"ConnectionID"`, `"Title"`, `"Summary"`} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("conversation response leaked server field %s: %s", forbidden, body)
		}
	}

	resp = h.do(t, http.MethodGet, "/api/connections/c-op/ai/conversations", "op", nil)
	if resp.Status != http.StatusOK {
		t.Fatalf("list conversations: %d (%s)", resp.Status, resp.Body)
	}
	body = string(resp.Body)
	if !strings.Contains(body, `"id":"`+convID+`"`) || !strings.Contains(body, `"title":"New conversation"`) {
		t.Fatalf("conversation list missing client keys: %s", body)
	}
	if strings.Contains(body, `"ID"`) || strings.Contains(body, `"Title"`) {
		t.Fatalf("conversation list used server keys: %s", body)
	}
}
