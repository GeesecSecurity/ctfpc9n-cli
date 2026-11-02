package cli

import (
	"bytes"
	"ctfpc9n-cli/internal/session"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestEvaluationReleaseHelpIsAvailableOfflineAndDocumentsLoginBase(t *testing.T) {
	for _, topic := range [][]string{{"evaluation"}, {"evaluation", "auth", "login"}, {"evaluation", "run", "start"}, {"evaluation", "report"}, {"challenge", "list"}} {
		var out bytes.Buffer
		if code := Execute(append([]string{"help"}, topic...), nil, &out); code != 0 {
			t.Fatalf("offline help %v: %s", topic, out.String())
		}
		var result struct {
			OK   bool            `json:"ok"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || !result.OK {
			t.Fatalf("invalid help: %s", out.String())
		}
		if strings.Join(topic, " ") == "evaluation auth login" {
			var help commandHelp
			if err := json.Unmarshal(result.Data, &help); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(help.Usage, "--api-base=URL") {
				t.Fatalf("evaluation login usage omits required API base: %s", help.Usage)
			}
		}
	}
}

func TestEvaluationHelpSchemasExposeResumeLockFields(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		fields []string
	}{
		{name: "challenge list", args: []string{"help", "evaluation", "challenge", "list"}, fields: []string{"locked", "lockedReason", "attackStatus"}},
		{name: "challenge get", args: []string{"help", "evaluation", "challenge", "get"}, fields: []string{"locked", "lockedReason", "attackStatus"}},
		{name: "submission flag", args: []string{"help", "evaluation", "submission", "flag"}, fields: []string{"alreadySolved"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if code := Execute(tc.args, nil, &out); code != 0 {
				t.Fatalf("help exit code = %d: %s", code, out.String())
			}
			var result struct {
				OK   bool            `json:"ok"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil || !result.OK {
				t.Fatalf("invalid help: %s", out.String())
			}
			var help commandHelp
			if err := json.Unmarshal(result.Data, &help); err != nil {
				t.Fatal(err)
			}
			schemaBytes, err := json.Marshal(help.ResultSchema)
			if err != nil {
				t.Fatal(err)
			}
			for _, field := range tc.fields {
				if !strings.Contains(string(schemaBytes), field) {
					t.Errorf("result schema missing %q: %s", field, string(schemaBytes))
				}
			}
		})
	}
}

func TestEvaluationRunStartHelpDocumentsExplicitResumeRule(t *testing.T) {
	var out bytes.Buffer
	if code := Execute([]string{"help", "evaluation", "run", "start"}, nil, &out); code != 0 {
		t.Fatalf("help exit code = %d: %s", code, out.String())
	}
	var result struct {
		OK   bool        `json:"ok"`
		Data commandHelp `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || !result.OK {
		t.Fatalf("invalid help: %s", out.String())
	}
	prerequisites := strings.Join(result.Data.Prerequisites, " ")
	for _, want := range []string{"completed", "timed_out", "explicitly", "new questions", "retest"} {
		if !strings.Contains(strings.ToLower(prerequisites), strings.ToLower(want)) {
			t.Errorf("prerequisites missing %q: %v", want, result.Data.Prerequisites)
		}
	}
}

func TestEvaluationLockedConflictIsNonRetryableAndPreservesMessage(t *testing.T) {
	const lockedMessage = "续测时段只开放新增或需重测的题目"
	t.Setenv("CTFPC9N_STATE_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":409,"msg":"` + lockedMessage + `"}`))
	}))
	defer server.Close()
	if _, err := session.Save("eval-lock", session.Value{Kind: "evaluation", APIBase: server.URL + "/evaluation/agent/v1", Token: "private"}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := Execute([]string{"--session", "eval-lock", "evaluation", "runtime", "start", "--run-id", "run-lock", "--challenge-id", "challenge-lock", "--request-id", "lock-0001"}, nil, &out); code != 1 {
		t.Fatalf("exit code = %d, output = %s", code, out.String())
	}
	var result struct {
		OK    bool `json:"ok"`
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			Retryable bool   `json:"retryable"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.OK || result.Error.Code != "CONFLICT" || result.Error.Retryable || result.Error.Message != "API error code=409: "+lockedMessage {
		t.Fatalf("unexpected error mapping: %s", out.String())
	}
}

func TestEvaluationSkillDocumentsResumeLockRules(t *testing.T) {
	data, err := os.ReadFile("../../skills/ctfpc9n-cli/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"explicitly requests", "locked: true", "alreadySolved: true", "kind: \"unavailable\"", "CONFLICT"} {
		if !strings.Contains(text, want) {
			t.Errorf("skill missing %q", want)
		}
	}
}
