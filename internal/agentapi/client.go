package agentapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	generated "ctfpc9n-cli/internal/generated/agentapi"
)

const (
	maxJSONResponseBytes = 8 << 20
	maxErrorBodyBytes    = 4 << 10
)

type APIError struct {
	HTTPStatus int
	Code       int
	Message    string
}

func (err *APIError) Error() string {
	if err.Code != 0 {
		return fmt.Sprintf("API error code=%d: %s", err.Code, err.Message)
	}
	return fmt.Sprintf("HTTP error status=%d: %s", err.HTTPStatus, err.Message)
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

type DownloadResult struct {
	Output      string `json:"output"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	Bytes       int64  `json:"bytes"`
}

func NormalizeBase(value string) (string, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid API base URL %q", value)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("API base URL must use http or https")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("API base URL must not contain credentials, query, or fragment")
	}
	path := strings.TrimRight(parsed.EscapedPath(), "/")
	if !strings.HasSuffix(path, "/agent/v1") {
		path += "/agent/v1"
	}
	parsed.Path = path
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func New(baseURL, token string, timeout time.Duration, insecureTLS bool) (*Client, error) {
	normalized, err := NormalizeBase(baseURL)
	if err != nil {
		return nil, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("Agent token is required")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: insecureTLS} //nolint:gosec // Explicit CLI development flag.
	return &Client{
		baseURL: normalized,
		token:   token,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}, nil
}

func (client *Client) Call(ctx context.Context, endpoint generated.Endpoint, request, response any, requestID string, sensitive ...string) error {
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, endpoint.Method, client.baseURL+endpoint.Path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+client.token)
	req.Header.Set("Content-Type", "application/json")
	if requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", endpoint.ContractPath, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxJSONResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if len(data) > maxJSONResponseBytes {
		return &APIError{HTTPStatus: resp.StatusCode, Message: "response exceeds 8 MiB limit"}
	}
	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return &APIError{HTTPStatus: resp.StatusCode, Message: nonJSONMessage(data)}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || (envelope.Code != 0 && envelope.Code != http.StatusOK) {
		return &APIError{HTTPStatus: resp.StatusCode, Code: envelope.Code, Message: redact(envelope.Msg, append(sensitive, client.token)...)}
	}
	if response == nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, response); err != nil {
		return fmt.Errorf("decode response data: %w", err)
	}
	return nil
}

func (client *Client) Download(ctx context.Context, endpoint generated.Endpoint, request any, output string) (DownloadResult, error) {
	absOutput, err := prepareOutput(output)
	if err != nil {
		return DownloadResult{}, err
	}
	body, err := json.Marshal(request)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, endpoint.Method, client.baseURL+endpoint.Path, bytes.NewReader(body))
	if err != nil {
		return DownloadResult{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+client.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("request %s: %w", endpoint.ContractPath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return DownloadResult{}, decodeDownloadError(resp, client.token)
	}
	if resp.StatusCode == http.StatusNoContent {
		return DownloadResult{}, &APIError{HTTPStatus: resp.StatusCode, Message: "attachment response has no content"}
	}
	if isJSONErrorResponse(resp) {
		return DownloadResult{}, decodeDownloadError(resp, client.token)
	}
	bytesWritten, err := writeOutput(absOutput, resp.Body)
	if err != nil {
		return DownloadResult{}, err
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if mediaType, _, parseErr := mime.ParseMediaType(contentType); parseErr == nil && mediaType != "" {
		contentType = mediaType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return DownloadResult{Output: absOutput, FileName: filepath.Base(absOutput), ContentType: contentType, Bytes: bytesWritten}, nil
}

func decodeDownloadError(response *http.Response, token string) error {
	data, err := io.ReadAll(io.LimitReader(response.Body, maxErrorBodyBytes+1))
	if err != nil {
		return fmt.Errorf("read error response: %w", err)
	}
	var envelope struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if json.Unmarshal(data, &envelope) == nil {
		return &APIError{HTTPStatus: response.StatusCode, Code: envelope.Code, Message: redact(envelope.Msg, token)}
	}
	return &APIError{HTTPStatus: response.StatusCode, Message: nonJSONMessage(data)}
}

func isJSONErrorResponse(response *http.Response) bool {
	if strings.TrimSpace(response.Header.Get("Content-Disposition")) != "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	return err == nil && mediaType == "application/json"
}

func prepareOutput(output string) (string, error) {
	if strings.TrimSpace(output) == "" {
		return "", errors.New("--output is required")
	}
	absOutput, err := filepath.Abs(output)
	if err != nil {
		return "", fmt.Errorf("resolve output path: %w", err)
	}
	if _, err := os.Lstat(absOutput); err == nil {
		return "", fmt.Errorf("output file already exists: %s", absOutput)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect output path: %w", err)
	}
	directory := filepath.Dir(absOutput)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}
	return absOutput, nil
}

func writeOutput(output string, body io.Reader) (int64, error) {
	directory := filepath.Dir(output)
	temporary, err := os.CreateTemp(directory, ".ctfpc9n-download-*")
	if err != nil {
		return 0, fmt.Errorf("create temporary output: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return 0, fmt.Errorf("secure temporary output: %w", err)
	}
	bytesWritten, copyErr := io.Copy(temporary, body)
	closeErr := temporary.Close()
	if copyErr != nil {
		return 0, fmt.Errorf("write output: %w", copyErr)
	}
	if closeErr != nil {
		return 0, fmt.Errorf("close output: %w", closeErr)
	}
	if err := os.Link(temporaryPath, output); err != nil {
		if errors.Is(err, os.ErrExist) {
			return 0, fmt.Errorf("output file already exists: %s", output)
		}
		return 0, fmt.Errorf("publish output without overwrite: %w", err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		return 0, fmt.Errorf("finalize output: %w", err)
	}
	return bytesWritten, nil
}

func nonJSONMessage(data []byte) string {
	message := strings.TrimSpace(string(data))
	if message == "" {
		return "response is not valid JSON"
	}
	if len(message) > maxErrorBodyBytes {
		message = message[:maxErrorBodyBytes] + "... (truncated)"
	}
	return "response is not valid JSON: " + message
}

func redact(message string, values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			message = strings.ReplaceAll(message, value, "[redacted]")
		}
	}
	for _, key := range []string{"token", "flag", "cookie", "password", "secret"} {
		message = redactMarkedValue(message, key+"=")
		message = redactMarkedValue(message, key+":")
	}
	return message
}

func redactMarkedValue(message, marker string) string {
	searchFrom := 0
	for searchFrom < len(message) {
		index := strings.Index(strings.ToLower(message[searchFrom:]), marker)
		if index < 0 {
			break
		}
		index += searchFrom
		valueStart := index + len(marker)
		if strings.HasPrefix(message[valueStart:], "[redacted]") {
			searchFrom = valueStart + len("[redacted]")
			continue
		}
		end := valueStart
		for end < len(message) && !strings.ContainsRune(" &\t\r\n\"'", rune(message[end])) {
			end++
		}
		message = message[:valueStart] + "[redacted]" + message[end:]
		searchFrom = valueStart + len("[redacted]")
	}
	return message
}

func ContentLength(response *http.Response) int64 {
	value := strings.TrimSpace(response.Header.Get("Content-Length"))
	if value == "" {
		return -1
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return -1
	}
	return parsed
}
