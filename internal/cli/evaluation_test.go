package cli

import (
	"bytes"
	"ctfpc9n-cli/internal/session"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEvaluationLoginAndCrossSessionIsolation(t *testing.T) {
	t.Setenv("CTFPC9N_STATE_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/evaluation/agent/v1/session" || r.Header.Get("Authorization") != "Bearer secret-eval-token" {
			t.Errorf("wrong scoped request: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":200,"data":{"account":{"id":"agent-1","name":"agent"}}}`))
	}))
	defer server.Close()
	var out bytes.Buffer
	status := Execute([]string{"--session", "eval-a", "evaluation", "auth", "login", "--api-base", server.URL, "--token-stdin"}, strings.NewReader("secret-eval-token\n"), &out)
	if status != 0 {
		t.Fatal(out.String())
	}
	if strings.Contains(out.String(), "secret-eval-token") {
		t.Fatal("token leaked")
	}
	v, e := session.Load("eval-a")
	if e != nil || v.Kind != "evaluation" {
		t.Fatalf("session kind: %v", e)
	}
	out.Reset()
	if Execute([]string{"--session", "eval-a", "challenge", "list"}, nil, &out) == 0 {
		t.Fatal("evaluation session accepted by competition")
	}
	var body map[string]any
	if json.Unmarshal(out.Bytes(), &body) != nil || body["ok"] != false {
		t.Fatal(out.String())
	}
}
func TestEvaluationRejectsCredentialArguments(t *testing.T) {
	var out bytes.Buffer
	if Execute([]string{"evaluation", "auth", "login", "--token", "unsafe"}, nil, &out) == 0 {
		t.Fatal("token argument accepted")
	}
}

func TestEvaluationFlagStdinUsesScopedEndpointAndNeverEchoesFlag(t *testing.T) {
	t.Setenv("CTFPC9N_STATE_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/evaluation/agent/v1/submission/flag" || r.Header.Get("Authorization") != "Bearer protected-ai-token" {
			t.Error("incorrect evaluation scope")
		}
		var body struct {
			Flag      string `json:"flag"`
			RequestID string `json:"requestId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Flag != "flag{private-fixture}" || body.RequestID != "submit-0001" {
			t.Error("stdin or idempotency lost")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":200,"data":{"correct":true,"alreadySolved":false}}`))
	}))
	defer server.Close()
	if _, err := session.Save("eval-a", session.Value{Kind: "evaluation", APIBase: server.URL + "/evaluation/agent/v1", Token: "protected-ai-token"}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	result := Execute([]string{"--session", "eval-a", "evaluation", "submission", "flag", "--run-id", "run-a", "--challenge-id", "challenge-a", "--request-id", "submit-0001", "--flag-stdin"}, strings.NewReader("flag{private-fixture}\n"), &out)
	if result != 0 {
		t.Fatal(out.String())
	}
	if strings.Contains(out.String(), "private-fixture") || strings.Contains(out.String(), "protected-ai-token") {
		t.Fatal("secret echoed")
	}
}

func TestEvaluationChallengePagination(t *testing.T) {
	t.Setenv("CTFPC9N_STATE_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "2" || r.URL.Query().Get("pageSize") != "25" {
			t.Errorf("missing pagination: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":200,"data":{"items":[],"total":1000,"page":2,"pageSize":25}}`))
	}))
	defer server.Close()
	if _, err := session.Save("eval-page", session.Value{Kind: "evaluation", APIBase: server.URL + "/evaluation/agent/v1", Token: "private"}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if Execute([]string{"--session", "eval-page", "evaluation", "challenge", "list", "--run-id", "run-page", "--page", "2", "--page-size", "25"}, nil, &out) != 0 {
		t.Fatal(out.String())
	}
	if !strings.Contains(out.String(), `"total":1000`) && !strings.Contains(out.String(), `"total": 1000`) {
		t.Fatal(out.String())
	}
}
