package agentapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contract "ctfpc9n-cli/internal/generated/agentapi"
)

func TestCallSendsBearerRequestAndDecodesData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/agent/v1/runtimes/start" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if request.Method != http.MethodPost {
			t.Errorf("method = %q", request.Method)
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("X-Request-Id") != "start-1" {
			t.Errorf("request ID = %q", request.Header.Get("X-Request-Id"))
		}
		_, _ = io.WriteString(writer, `{"code":0,"msg":"ok","data":{"address":"https://runtime.example","rest_time":60,"container_id":"c-1"}}`)
	}))
	defer server.Close()

	client, err := New(server.URL, "test-token", time.Second, false)
	if err != nil {
		t.Fatal(err)
	}
	var response contract.StartChallengeResponse
	err = client.Call(context.Background(), contract.EndpointAgentStartRuntime, &contract.AgentRuntimeRequest{ChallengeId: 1, RequestId: "start-1"}, &response, "start-1")
	if err != nil {
		t.Fatal(err)
	}
	if response.ContainerId != "c-1" || response.RestTime != 60 {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestCallRedactsTokenAndFlagInAPIError(t *testing.T) {
	const token = "super-secret-token"
	const flag = "FLAG{private}"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(writer, `{"code":400,"msg":"token=super-secret-token flag=FLAG{private}"}`)
	}))
	defer server.Close()

	client, err := New(server.URL, token, time.Second, false)
	if err != nil {
		t.Fatal(err)
	}
	err = client.Call(context.Background(), contract.EndpointAgentSubmitFlag, &contract.AgentFlagSubmitRequest{ChallengeId: 1, Flag: flag, RequestId: "submit-1"}, nil, "submit-1", flag)
	if err == nil {
		t.Fatal("Call() succeeded unexpectedly")
	}
	if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), flag) {
		t.Fatalf("sensitive values leaked in %q", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != http.StatusBadRequest {
		t.Fatalf("error = %#v", err)
	}
}

func TestDownloadCreatesPrivateNewFileWithoutOverwrite(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		writer.Header().Set("Content-Type", "application/zip")
		_, _ = io.WriteString(writer, "zip-bytes")
	}))
	defer server.Close()
	client, err := New(server.URL, "test-token", time.Second, false)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	output := filepath.Join(directory, "nested", "task.zip")
	result, err := client.Download(context.Background(), contract.EndpointAgentDownloadAttachment, &contract.AgentAttachmentDownloadRequest{ChallengeId: 1, AttachmentPath: "/task.zip"}, output)
	if err != nil {
		t.Fatal(err)
	}
	if result.Output != output || result.Bytes != int64(len("zip-bytes")) || result.ContentType != "application/zip" {
		t.Fatalf("result = %#v", result)
	}
	content, err := os.ReadFile(output)
	if err != nil || string(content) != "zip-bytes" {
		t.Fatalf("content = %q, err = %v", content, err)
	}
	info, err := os.Stat(output)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %o", info.Mode().Perm())
	}
	_, err = client.Download(context.Background(), contract.EndpointAgentDownloadAttachment, &contract.AgentAttachmentDownloadRequest{ChallengeId: 1, AttachmentPath: "/task.zip"}, output)
	if err == nil {
		t.Fatal("overwrite download succeeded")
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestDownloadRejectsHTTP200BusinessErrorJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"code":402,"msg":"attachment is not ready"}`)
	}))
	defer server.Close()
	client, err := New(server.URL, "test-token", time.Second, false)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "task.zip")
	_, err = client.Download(context.Background(), contract.EndpointAgentDownloadAttachment, &contract.AgentAttachmentDownloadRequest{ChallengeId: 1, AttachmentPath: "/task.zip"}, output)
	if err == nil {
		t.Fatal("Download() accepted a JSON business error")
	}
	if _, statErr := os.Stat(output); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output stat error = %v, want not exist", statErr)
	}
}
