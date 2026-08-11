package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"ctfpc9n-cli/internal/agentapi"
	"ctfpc9n-cli/internal/session"

	"github.com/alecthomas/kong"
)

const schemaVersion = "v1"

const (
	defaultHTTPTimeout = 15 * time.Second
	maxWaitSeconds     = 30
	waitTimeoutGrace   = 5 * time.Second
)

var (
	errUsage = errors.New("invalid command arguments")
	errAuth  = errors.New("authentication required")
)

type Options struct {
	Session     string
	APIBase     string
	APIBaseSet  bool
	Timeout     time.Duration
	TimeoutSet  bool
	InsecureTLS bool
}

type resultError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type result struct {
	SchemaVersion string       `json:"schemaVersion"`
	OK            bool         `json:"ok"`
	Command       string       `json:"command"`
	Data          any          `json:"data,omitempty"`
	Error         *resultError `json:"error,omitempty"`
}

func Execute(args []string, stdin io.Reader, stdout io.Writer) int {
	options, remaining, err := extractOptions(args)
	if err == nil {
		err = runWithOptions(options, remaining, stdin, stdout)
	}
	if err == nil {
		return 0
	}
	writeFailure(stdout, commandPath(remaining), err)
	if errors.Is(err, errUsage) {
		return 2
	}
	return 1
}

func Run(args []string, stdin io.Reader, stdout io.Writer) error {
	options, remaining, err := extractOptions(args)
	if err != nil {
		return err
	}
	return runWithOptions(options, remaining, stdin, stdout)
}

func extractOptions(args []string) (Options, []string, error) {
	options := Options{Timeout: defaultHTTPTimeout}
	remaining := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		argument := args[index]
		name, value, inline := strings.Cut(argument, "=")
		next := func() (string, error) {
			if inline {
				return value, nil
			}
			index++
			if index >= len(args) {
				return "", usagef("%s requires a value", name)
			}
			return args[index], nil
		}
		switch name {
		case "--session":
			value, err := next()
			if err != nil {
				return Options{}, nil, err
			}
			options.Session = strings.TrimSpace(value)
		case "--api-base":
			value, err := next()
			if err != nil {
				return Options{}, nil, err
			}
			options.APIBase = strings.TrimSpace(value)
			options.APIBaseSet = true
		case "--timeout":
			value, err := next()
			if err != nil {
				return Options{}, nil, err
			}
			timeout, err := time.ParseDuration(value)
			if err != nil || timeout <= 0 {
				return Options{}, nil, usagef("--timeout must be a positive duration")
			}
			options.Timeout = timeout
			options.TimeoutSet = true
		case "--insecure-skip-tls-verify", "-K":
			if inline {
				remaining = append(remaining, argument)
				continue
			}
			options.InsecureTLS = true
		default:
			remaining = append(remaining, argument)
		}
	}
	return options, remaining, nil
}

func runWithOptions(options Options, args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		return writeHelp(stdout, nil)
	}
	if args[0] == "help" {
		return writeHelp(stdout, args[1:])
	}
	if topic, ok := inlineHelpTopic(args); ok {
		return writeHelp(stdout, topic)
	}
	schema, command, err := parseCommand(args)
	if err != nil {
		return err
	}
	applyWaitTimeout(&options, command, schema)
	return runCommand(context.Background(), options, schema, command, stdin, stdout)
}

func applyWaitTimeout(options *Options, command string, schema *commandSchema) {
	if options.TimeoutSet {
		return
	}
	var waitSeconds uint64
	switch command {
	case "attachment dynamic status":
		waitSeconds = schema.Attachment.Dynamic.Status.WaitSeconds
	case "runtime inspect":
		waitSeconds = schema.Runtime.Inspect.WaitSeconds
	default:
		return
	}
	if waitSeconds > maxWaitSeconds {
		return
	}
	minimum := time.Duration(waitSeconds)*time.Second + waitTimeoutGrace
	if minimum > options.Timeout {
		options.Timeout = minimum
	}
}

func parseCommand(args []string) (*commandSchema, string, error) {
	schema := new(commandSchema)
	parser, err := kong.New(schema, kong.Name("ctfpc9n-cli"), kong.Writers(io.Discard, io.Discard))
	if err != nil {
		return nil, "", fmt.Errorf("initialize command schema: %w", err)
	}
	context, err := parser.Parse(args)
	if err != nil {
		return nil, "", usagef("%v", err)
	}
	return schema, context.Command(), nil
}

func inlineHelpTopic(args []string) ([]string, bool) {
	pathEnd := 0
	for pathEnd < len(args) && !strings.HasPrefix(args[pathEnd], "-") {
		pathEnd++
	}
	for _, argument := range args[pathEnd:] {
		if argument == "--help" || argument == "-h" {
			return args[:pathEnd], true
		}
	}
	return nil, false
}

func newClient(options Options) (*agentapi.Client, error) {
	if options.Session == "" {
		return nil, usagef("--session is required")
	}
	if options.APIBaseSet {
		return nil, usagef("--api-base is only valid with auth login")
	}
	stored, err := session.Load(options.Session)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errAuth, err)
	}
	client, err := agentapi.New(stored.APIBase, stored.Token, options.Timeout, options.InsecureTLS)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errAuth, err)
	}
	return client, nil
}

func validateSessionOption(options Options) error {
	if options.Session == "" {
		return usagef("--session is required")
	}
	return session.ValidateName(options.Session)
}

func readStdinLine(stdin io.Reader, label string) (string, error) {
	reader := bufio.NewReader(stdin)
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read %s: %w", label, err)
	}
	value = strings.TrimSuffix(value, "\n")
	value = strings.TrimSuffix(value, "\r")
	if strings.TrimSpace(value) == "" {
		return "", usagef("%s stdin value must not be empty", label)
	}
	return value, nil
}

func writeResult(stdout io.Writer, command string, data any) error {
	encoded, err := json.Marshal(result{SchemaVersion: schemaVersion, OK: true, Command: command, Data: data})
	if err != nil {
		return fmt.Errorf("encode result: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(encoded))
	return err
}

func writeFailure(stdout io.Writer, command string, cause error) {
	detail := classifyError(cause)
	encoded, err := json.Marshal(result{SchemaVersion: schemaVersion, OK: false, Command: command, Error: &detail})
	if err == nil {
		_, _ = fmt.Fprintln(stdout, string(encoded))
	}
}

func classifyError(cause error) resultError {
	message := agentapi.Redact(cause.Error())
	if errors.Is(cause, errUsage) {
		return resultError{Code: "USAGE", Message: message}
	}
	if errors.Is(cause, errAuth) {
		return resultError{Code: "AUTH_REQUIRED", Message: message}
	}
	var apiErr *agentapi.APIError
	if errors.As(cause, &apiErr) {
		status := apiErr.HTTPStatus
		if apiErr.Code >= 400 && apiErr.Code <= 599 {
			status = apiErr.Code
		}
		switch status {
		case 401:
			return resultError{Code: "AUTH_REQUIRED", Message: message}
		case 403:
			return resultError{Code: "PERMISSION_DENIED", Message: message}
		case 404:
			return resultError{Code: "NOT_FOUND", Message: message}
		case 409:
			return resultError{Code: "CONFLICT", Message: message}
		case 408, 429:
			return resultError{Code: "UNAVAILABLE", Message: message, Retryable: true}
		default:
			if status >= 500 {
				return resultError{Code: "UNAVAILABLE", Message: message, Retryable: true}
			}
			return resultError{Code: "API_ERROR", Message: message}
		}
	}
	if errors.Is(cause, context.DeadlineExceeded) || strings.Contains(strings.ToLower(message), "timeout") || strings.HasPrefix(strings.ToLower(message), "request ") {
		return resultError{Code: "UNAVAILABLE", Message: message, Retryable: true}
	}
	return resultError{Code: "INTERNAL", Message: message}
}

func usagef(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{errUsage}, args...)...)
}

func commandPath(args []string) string {
	parts := make([]string, 0, 3)
	for _, argument := range args {
		if strings.HasPrefix(argument, "-") || len(parts) == 3 {
			break
		}
		parts = append(parts, argument)
	}
	return strings.Join(parts, " ")
}

func isPositive(value uint64, name string) error {
	if value == 0 {
		return usagef("%s must be positive", name)
	}
	return nil
}

func isPage(value uint64, name string) error {
	return isPositive(value, name)
}

func isWait(value uint64) error {
	if value > maxWaitSeconds {
		return usagef("--wait-seconds must be between 0 and 30")
	}
	return nil
}

func validateRequestID(value string) (string, error) {
	if len(value) == 0 || len(value) > 64 {
		return "", usagef("--request-id must be 1 to 64 characters")
	}
	for index, char := range value {
		valid := char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '.' || char == '_' || char == '-'
		if !valid || (index == 0 && !(char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9')) {
			return "", usagef("--request-id must match [A-Za-z0-9][A-Za-z0-9._-]{0,63}")
		}
	}
	return value, nil
}

func stdinOrDefault(stdin io.Reader) io.Reader {
	if stdin == nil {
		return os.Stdin
	}
	return stdin
}
