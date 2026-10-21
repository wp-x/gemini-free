package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIAndProbeReportAccountRequirementInAnonymousMode(t *testing.T) {
	controller, _ := testController(t)
	control := testControl(t, controller)
	const expected = "此模型需要登录会话，当前为匿名模式"
	for _, path := range []string{"/v1/chat/completions", "/v1/messages"} {
		body := `{"model":"gemini-4-argon-high","messages":[{"role":"user","content":"hi"}]}`
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		response := controlResponse(controller, request)
		expectStatus(t, response, http.StatusBadGateway)
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("%s: missing login requirement: %s", path, response.Body.String())
		}
	}
	response := controlJSON(control, "/control/probe", map[string]string{
		"model": "gemini-4-argon-high", "prompt": "hi",
	})
	if !strings.Contains(response.Body.String(), "event: error") ||
		!strings.Contains(response.Body.String(), expected) {
		t.Fatalf("probe missing login requirement: %s", response.Body.String())
	}
}
