package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"ctfpc9n-cli/internal/session"
)

func TestWaitCommandsExtendOnlyTheImplicitHTTPTimeout(t *testing.T) {
	cases := []struct {
		name       string
		command    string
		wait       uint64
		timeoutSet bool
		want       time.Duration
	}{
		{name: "runtime inspect", command: "runtime inspect", wait: 30, want: 35 * time.Second},
		{name: "dynamic attachment status", command: "attachment dynamic status", wait: 30, want: 35 * time.Second},
		{name: "zero wait retains default", command: "runtime inspect", want: defaultHTTPTimeout},
		{name: "explicit timeout wins", command: "runtime inspect", wait: 30, timeoutSet: true, want: defaultHTTPTimeout},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			options := Options{Timeout: defaultHTTPTimeout, TimeoutSet: testCase.timeoutSet}
			schema := new(commandSchema)
			schema.Runtime.Inspect.WaitSeconds = testCase.wait
			schema.Attachment.Dynamic.Status.WaitSeconds = testCase.wait

			applyWaitTimeout(&options, testCase.command, schema)

			if options.Timeout != testCase.want {
				t.Fatalf("timeout = %s, want %s", options.Timeout, testCase.want)
			}
		})
	}
}

func TestAuthLoginValidatesTokenAndDoesNotPrintIt(t *testing.T) {
	const token = "token-not-for-output"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/agent/v1/challenges/list" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer "+token {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(writer, `{"code":0,"msg":"ok","data":{"challenges":[],"dark_mode":false}}`)
	}))
	defer server.Close()
	t.Setenv("CTFPC9N_STATE_DIR", filepath.Join(t.TempDir(), "state"))
	t.Setenv(agentTokenEnvironment, "environment-token-not-used")
	var output bytes.Buffer
	exitCode := Execute([]string{"--session", "contest-a", "auth", "login", "--api-base", server.URL, "--token-stdin"}, strings.NewReader(token+"\n"), &output)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, output = %s", exitCode, output.String())
	}
	if strings.Contains(output.String(), token) {
		t.Fatalf("token leaked in %s", output.String())
	}
	stored, err := session.Load("contest-a")
	if err != nil || stored.Token != token {
		t.Fatalf("stored session = %#v, err = %v", stored, err)
	}
}

func TestAuthLoginSupportsTokenFlagAndEnvironment(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		envToken string
		want     string
	}{
		{name: "token flag overrides environment", args: []string{"--token", "flag-token"}, envToken: "environment-token", want: "flag-token"},
		{name: "environment fallback", envToken: "environment-token", want: "environment-token"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Header.Get("Authorization") != "Bearer "+testCase.want {
					t.Errorf("authorization = %q", request.Header.Get("Authorization"))
				}
				_, _ = io.WriteString(writer, `{"code":0,"msg":"ok","data":{"challenges":[],"dark_mode":false}}`)
			}))
			defer server.Close()
			t.Setenv("CTFPC9N_STATE_DIR", filepath.Join(t.TempDir(), "state"))
			t.Setenv(agentTokenEnvironment, testCase.envToken)
			args := []string{"--session", "contest-a", "auth", "login", "--api-base", server.URL}
			args = append(args, testCase.args...)
			var output bytes.Buffer
			if code := Execute(args, strings.NewReader(""), &output); code != 0 {
				t.Fatalf("exit code = %d, output = %s", code, output.String())
			}
			for _, secret := range []string{testCase.want, testCase.envToken} {
				if secret != "" && strings.Contains(output.String(), secret) {
					t.Fatalf("token leaked in %s", output.String())
				}
			}
			stored, err := session.Load("contest-a")
			if err != nil || stored.Token != testCase.want {
				t.Fatalf("stored session = %#v, err = %v", stored, err)
			}
		})
	}
}

func TestAuthLoginRejectsInvalidTokenSourcesWithoutLeaking(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		stdin    string
		envToken string
		secrets  []string
	}{
		{name: "missing source"},
		{name: "empty explicit token does not fall back", args: []string{"--token", ""}, envToken: "environment-token", secrets: []string{"environment-token"}},
		{name: "conflicting explicit sources", args: []string{"--token", "argument-token", "--token-stdin"}, stdin: "stdin-token\n", envToken: "environment-token", secrets: []string{"argument-token", "stdin-token", "environment-token"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				requests++
				_, _ = io.WriteString(writer, `{"code":0,"msg":"ok","data":{"challenges":[],"dark_mode":false}}`)
			}))
			defer server.Close()
			t.Setenv("CTFPC9N_STATE_DIR", filepath.Join(t.TempDir(), "state"))
			t.Setenv(agentTokenEnvironment, testCase.envToken)
			args := []string{"--session", "contest-a", "auth", "login", "--api-base", server.URL}
			args = append(args, testCase.args...)
			var output bytes.Buffer
			if code := Execute(args, strings.NewReader(testCase.stdin), &output); code != 2 {
				t.Fatalf("exit code = %d, output = %s", code, output.String())
			}
			if requests != 0 {
				t.Fatalf("requests = %d, want 0", requests)
			}
			for _, secret := range testCase.secrets {
				if strings.Contains(output.String(), secret) {
					t.Fatalf("token leaked in %s", output.String())
				}
			}
			if _, err := session.Load("contest-a"); err == nil {
				t.Fatal("invalid token source saved a session")
			}
		})
	}
}

func TestAuthLoginHelpDocumentsAllTokenSources(t *testing.T) {
	var output bytes.Buffer
	if code := Execute([]string{"help", "auth", "login"}, strings.NewReader(""), &output); code != 0 {
		t.Fatalf("exit code = %d, output = %s", code, output.String())
	}
	for _, expected := range []string{"--token", "--token-stdin", agentTokenEnvironment, `"sensitive":true`} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("help is missing %q: %s", expected, output.String())
		}
	}
}

func TestAuthLoginFailureDoesNotSaveSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.WriteString(writer, `{"code":401,"msg":"invalid token"}`)
	}))
	defer server.Close()
	t.Setenv("CTFPC9N_STATE_DIR", filepath.Join(t.TempDir(), "state"))
	var output bytes.Buffer
	exitCode := Execute([]string{"--session", "contest-a", "auth", "login", "--api-base", server.URL, "--token-stdin"}, strings.NewReader("private-token\n"), &output)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, output = %s", exitCode, output.String())
	}
	if strings.Contains(output.String(), "private-token") {
		t.Fatalf("token leaked in %s", output.String())
	}
	if _, err := session.Load("contest-a"); err == nil {
		t.Fatal("failed login saved a session")
	}
}

func TestAuthLogoutDeletesSessionAndIsIdempotent(t *testing.T) {
	t.Setenv("CTFPC9N_STATE_DIR", filepath.Join(t.TempDir(), "state"))
	if _, err := session.Save("contest-a", session.Value{APIBase: "https://competition.example", Token: "test-token"}); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		var output bytes.Buffer
		if code := Execute([]string{"--session", "contest-a", "auth", "logout"}, strings.NewReader(""), &output); code != 0 {
			t.Fatalf("logout attempt %d exit code = %d, output = %s", attempt, code, output.String())
		}
	}
	if _, err := session.Load("contest-a"); err == nil {
		t.Fatal("logout did not delete the session")
	}
}

func TestRuntimeStartSendsOneRequestWithoutImplicitInspect(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.URL.Path)
		if request.Header.Get("X-Request-Id") != "start-1" {
			t.Errorf("request ID = %q", request.Header.Get("X-Request-Id"))
		}
		_, _ = io.WriteString(writer, `{"code":0,"msg":"ok","data":{"address":"https://runtime.example","rest_time":60,"container_id":"c-1"}}`)
	}))
	defer server.Close()
	t.Setenv("CTFPC9N_STATE_DIR", filepath.Join(t.TempDir(), "state"))
	if _, err := session.Save("contest-a", session.Value{APIBase: server.URL, Token: "test-token"}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	exitCode := Execute([]string{"--session", "contest-a", "runtime", "start", "--challenge-id", "1", "--request-id", "start-1"}, strings.NewReader(""), &output)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, output = %s", exitCode, output.String())
	}
	if len(paths) != 1 || paths[0] != "/agent/v1/runtimes/start" {
		t.Fatalf("paths = %#v", paths)
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			ContainerID string `json:"container_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil || !envelope.OK || envelope.Data.ContainerID != "c-1" {
		t.Fatalf("output = %s, err = %v", output.String(), err)
	}
}

func TestEveryLeafHelpFormReturnsIdenticalJSON(t *testing.T) {
	for _, spec := range commandCatalog() {
		t.Run(spec.Path, func(t *testing.T) {
			var explicit, inline bytes.Buffer
			path := strings.Fields(spec.Path)
			if code := Execute(append([]string{"help"}, path...), strings.NewReader(""), &explicit); code != 0 {
				t.Fatalf("help exit code = %d", code)
			}
			if code := Execute(append(path, "--help"), strings.NewReader(""), &inline); code != 0 {
				t.Fatalf("inline help exit code = %d", code)
			}
			if explicit.String() != inline.String() {
				t.Fatalf("help output differs\nexplicit: %s\ninline: %s", explicit.String(), inline.String())
			}
			var result struct {
				OK   bool                       `json:"ok"`
				Data map[string]json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(explicit.Bytes(), &result); err != nil || !result.OK {
				t.Fatalf("help output = %s, err = %v", explicit.String(), err)
			}
			for _, field := range []string{"path", "usage", "flags", "globalFlags", "request", "resultSchema"} {
				if len(result.Data[field]) == 0 || string(result.Data[field]) == "null" {
					t.Errorf("missing help field %q in %s", field, explicit.String())
				}
			}
		})
	}
}

func TestSensitiveAPIErrorsReturnRedactedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.WriteString(writer, `{"code":402,"msg":"flag=FLAG{private}"}`)
	}))
	defer server.Close()
	t.Setenv("CTFPC9N_STATE_DIR", filepath.Join(t.TempDir(), "state"))
	if _, err := session.Save("contest-a", session.Value{APIBase: server.URL, Token: "test-token"}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	exitCode := Execute([]string{"--session", "contest-a", "submission", "flag", "--challenge-id", "1", "--request-id", "submit-1", "--flag-stdin"}, strings.NewReader("FLAG{private}\n"), &output)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, output = %s", exitCode, output.String())
	}
	if strings.Contains(output.String(), "FLAG{private}") || !strings.Contains(output.String(), "[redacted]") {
		t.Fatalf("sensitive error output = %s", output.String())
	}
}

func TestParticipantCommandsUseOnlyManifestEndpoints(t *testing.T) {
	type testCase struct {
		name      string
		args      []string
		stdin     string
		path      string
		body      map[string]any
		requestID string
		response  string
		download  bool
	}
	cases := []testCase{
		{name: "stage progress", args: []string{"stage", "progress"}, path: "/agent/v1/stage/progress", response: `{"code":0,"msg":"ok","data":{"enabled":true,"supported":true,"graph":{"nodes":[],"edges":[]},"valid":true,"stageProgress":[],"unlockedStages":[]}}`},
		{name: "challenge list", args: []string{"challenge", "list"}, path: "/agent/v1/challenges/list", body: map[string]any{"tags": nil, "type": float64(0)}, response: `{"code":0,"msg":"ok","data":{"challenges":[],"dark_mode":false}}`},
		{name: "challenge get", args: []string{"challenge", "get", "--challenge-id", "1"}, path: "/agent/v1/challenges/detail", body: map[string]any{"challengeId": float64(1)}, response: `{"code":0,"msg":"ok","data":{"challenge":{},"dark_mode":false}}`},
		{name: "attachment download", args: []string{"attachment", "download", "--challenge-id", "1", "--attachment-path", "/task.zip"}, path: "/agent/v1/attachments/download", body: map[string]any{"challengeId": float64(1), "attachmentPath": "/task.zip"}, download: true},
		{name: "dynamic attachment status", args: []string{"attachment", "dynamic", "status", "--challenge-id", "1"}, path: "/agent/v1/attachments/dynamic/status", body: map[string]any{"challengeId": float64(1), "regenerate": false, "waitSeconds": float64(0)}, response: `{"code":0,"msg":"ok","data":{"challengeId":1,"status":"ready","message":"ok","downloadAvailable":true}}`},
		{name: "dynamic attachment download", args: []string{"attachment", "dynamic", "download", "--challenge-id", "1"}, path: "/agent/v1/attachments/dynamic/download", body: map[string]any{"challengeId": float64(1)}, download: true},
		{name: "runtime start", args: []string{"runtime", "start", "--challenge-id", "1", "--request-id", "write-1"}, path: "/agent/v1/runtimes/start", body: map[string]any{"challengeId": float64(1), "requestId": "write-1"}, requestID: "write-1", response: `{"code":0,"msg":"ok","data":{"address":"https://runtime.example","rest_time":60,"container_id":"c-1"}}`},
		{name: "runtime inspect", args: []string{"runtime", "inspect", "--challenge-id", "1"}, path: "/agent/v1/runtimes/inspect", body: map[string]any{"challengeId": float64(1), "waitSeconds": float64(0)}, response: `{"code":0,"msg":"ok","data":{"challengeId":1,"runtimeMode":"docker","status":"ready","reason":"","message":"ok","endpoints":[]}}`},
		{name: "runtime stop", args: []string{"runtime", "stop", "--challenge-id", "1", "--request-id", "write-1"}, path: "/agent/v1/runtimes/stop", body: map[string]any{"challengeId": float64(1), "requestId": "write-1"}, requestID: "write-1", response: `{"code":0,"msg":"ok","data":{"result":true}}`},
		{name: "runtime extend", args: []string{"runtime", "extend", "--challenge-id", "1", "--request-id", "write-1"}, path: "/agent/v1/runtimes/extend", body: map[string]any{"challengeId": float64(1), "requestId": "write-1"}, requestID: "write-1", response: `{"code":0,"msg":"ok","data":{"result":true,"message":"extended","restTime":60}}`},
		{name: "flag submission", args: []string{"submission", "flag", "--challenge-id", "1", "--request-id", "write-1", "--flag-stdin"}, stdin: "FLAG{test}\n", path: "/agent/v1/submissions/flag", body: map[string]any{"challengeId": float64(1), "flag": "FLAG{test}", "requestId": "write-1"}, requestID: "write-1", response: `{"code":0,"msg":"ok","data":{"result":true}}`},
		{name: "rank list", args: []string{"rank", "list"}, path: "/agent/v1/rank", body: map[string]any{"page": float64(1), "size": float64(100)}, response: `{"code":0,"msg":"ok","data":{"rank_list":[],"page_index":1,"total":0,"dark_mode":false}}`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodPost || request.URL.Path != testCase.path {
					t.Errorf("request = %s %s, want POST %s", request.Method, request.URL.Path, testCase.path)
				}
				if request.Header.Get("Authorization") != "Bearer test-token" {
					t.Errorf("authorization = %q", request.Header.Get("Authorization"))
				}
				if request.Header.Get("X-Request-Id") != testCase.requestID {
					t.Errorf("request ID = %q, want %q", request.Header.Get("X-Request-Id"), testCase.requestID)
				}
				var body map[string]any
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if !reflect.DeepEqual(body, testCase.body) {
					t.Errorf("body = %#v, want %#v", body, testCase.body)
				}
				if testCase.download {
					writer.Header().Set("Content-Type", "application/zip")
					writer.Header().Set("Content-Disposition", `attachment; filename="task.zip"`)
					_, _ = io.WriteString(writer, "zip-bytes")
					return
				}
				_, _ = io.WriteString(writer, testCase.response)
			}))
			defer server.Close()
			t.Setenv("CTFPC9N_STATE_DIR", filepath.Join(t.TempDir(), "state"))
			if _, err := session.Save("contest-a", session.Value{APIBase: server.URL, Token: "test-token"}); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"--session", "contest-a"}, testCase.args...)
			if testCase.download {
				args = append(args, "--output", filepath.Join(t.TempDir(), "task.zip"))
			}
			var output bytes.Buffer
			if code := Execute(args, strings.NewReader(testCase.stdin), &output); code != 0 {
				t.Fatalf("exit code = %d, output = %s", code, output.String())
			}
			var result struct {
				OK bool `json:"ok"`
			}
			if err := json.Unmarshal(output.Bytes(), &result); err != nil || !result.OK {
				t.Fatalf("output = %s, err = %v", output.String(), err)
			}
		})
	}
}
