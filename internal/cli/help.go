package cli

import (
	"bytes"
	"io"
	"reflect"
	"sort"
	"strings"

	internalapi "ctfpc9n-cli/internal/agentapi"
	contract "ctfpc9n-cli/internal/generated/agentapi"

	"github.com/alecthomas/kong"
)

const jsonSchemaDraft2020 = "https://json-schema.org/draft/2020-12/schema"

type commandSpec struct {
	Path          string
	Summary       string
	Prerequisites []string
	Endpoint      *contract.Endpoint
	Request       any
	Result        any
	RequestID     string
	Stdin         string
	Local         bool
}

type helpFlag struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
	Sensitive   bool   `json:"sensitive,omitempty"`
}

type commandDirectoryEntry struct {
	Path    string `json:"path"`
	Summary string `json:"summary"`
}

type commandHelp struct {
	Path          string         `json:"path"`
	Summary       string         `json:"summary"`
	Usage         string         `json:"usage"`
	Prerequisites []string       `json:"prerequisites"`
	GlobalFlags   []helpFlag     `json:"globalFlags"`
	Flags         []helpFlag     `json:"flags"`
	Request       map[string]any `json:"request"`
	ResultSchema  map[string]any `json:"resultSchema"`
}

type authLoginResult struct {
	Session   string `json:"session"`
	APIBase   string `json:"apiBase"`
	CreatedAt string `json:"createdAt"`
}

type authLogoutResult struct {
	Deleted bool `json:"deleted"`
}

func commandCatalog() []commandSpec {
	return []commandSpec{
		{Path: "auth login", Summary: "Validate a stdin token and persist a named local session.", Prerequisites: []string{"A supplied --api-base URL and --token-stdin."}, Endpoint: &contract.EndpointAgentListChallenges, Request: &contract.AgentChallengeListRequest{}, Result: &authLoginResult{}, Stdin: "--token-stdin"},
		{Path: "auth logout", Summary: "Delete a named local session; succeeds when it is already absent.", Prerequisites: []string{"A session name."}, Result: &authLogoutResult{}, Local: true},
		{Path: "stage progress", Summary: "Read the current team's stage graph and unlock progress.", Prerequisites: []string{"An authenticated session.", "Run before reading challenges when stage mode may be enabled."}, Endpoint: &contract.EndpointAgentGetStageProgress, Result: &contract.StageProgressResponse{}},
		{Path: "challenge list", Summary: "List participant-visible challenges.", Prerequisites: []string{"An authenticated session.", "Read stage progress first when stage mode is enabled."}, Endpoint: &contract.EndpointAgentListChallenges, Request: &contract.AgentChallengeListRequest{}, Result: &contract.GetChallengeListResponse{}},
		{Path: "challenge get", Summary: "Get participant-visible challenge details and submit logs.", Prerequisites: []string{"An authenticated session.", "The challenge must be unlocked in stage progress when stage mode is enabled."}, Endpoint: &contract.EndpointAgentGetChallenge, Request: &contract.AgentChallengeDetailRequest{}, Result: &contract.GetChallengeDetailInfoResponse{}},
		{Path: "attachment download", Summary: "Download a declared static attachment to a new local path.", Prerequisites: []string{"An authenticated session.", "attachment_path returned by challenge get."}, Endpoint: &contract.EndpointAgentDownloadAttachment, Request: &contract.AgentAttachmentDownloadRequest{}, Result: &internalapi.DownloadResult{}},
		{Path: "attachment dynamic status", Summary: "Read or request dynamic attachment generation status.", Prerequisites: []string{"An authenticated session.", "--request-id when --regenerate is supplied."}, Endpoint: &contract.EndpointAgentGetDynamicAttachment, Request: &contract.AgentDynamicAttachmentRequest{}, Result: &contract.AgentDynamicAttachmentStatusResponse{}, RequestID: "Audit correlation only; regeneration retries are not idempotent."},
		{Path: "attachment dynamic download", Summary: "Download a ready dynamic attachment to a new local path.", Prerequisites: []string{"An authenticated session.", "A ready status from attachment dynamic status."}, Endpoint: &contract.EndpointAgentDownloadDynamicAttachment, Request: &contract.AgentDynamicAttachmentDownloadRequest{}, Result: &internalapi.DownloadResult{}},
		{Path: "runtime start", Summary: "Start a challenge runtime; inspect its status explicitly afterward.", Prerequisites: []string{"An authenticated session.", "A stable --request-id."}, Endpoint: &contract.EndpointAgentStartRuntime, Request: &contract.AgentRuntimeRequest{}, Result: &contract.StartChallengeResponse{}, RequestID: "Stable idempotency request ID."},
		{Path: "runtime inspect", Summary: "Inspect a challenge runtime without changing it.", Prerequisites: []string{"An authenticated session."}, Endpoint: &contract.EndpointAgentInspectRuntime, Request: &contract.AgentRuntimeInspectRequest{}, Result: &contract.AgentRuntimeInspectResponse{}},
		{Path: "runtime stop", Summary: "Stop a challenge runtime.", Prerequisites: []string{"An authenticated session.", "A stable --request-id."}, Endpoint: &contract.EndpointAgentStopRuntime, Request: &contract.AgentRuntimeRequest{}, Result: &contract.BoolResp{}, RequestID: "Stable idempotency request ID."},
		{Path: "runtime extend", Summary: "Extend a challenge runtime.", Prerequisites: []string{"An authenticated session.", "A stable --request-id."}, Endpoint: &contract.EndpointAgentExtendRuntime, Request: &contract.AgentRuntimeRequest{}, Result: &contract.DelayChallengeResponse{}, RequestID: "Stable idempotency request ID."},
		{Path: "submission flag", Summary: "Submit one candidate Flag read from stdin.", Prerequisites: []string{"An authenticated session.", "A stable --request-id and --flag-stdin."}, Endpoint: &contract.EndpointAgentSubmitFlag, Request: &contract.AgentFlagSubmitRequest{}, Result: &contract.BoolResp{}, RequestID: "Stable idempotency request ID.", Stdin: "--flag-stdin"},
		{Path: "rank list", Summary: "List the competition rank.", Prerequisites: []string{"An authenticated session."}, Endpoint: &contract.EndpointAgentGetRank, Request: &contract.AgentRankRequest{}, Result: &contract.GetRankInfoResponse{}},
	}
}

func writeHelp(stdout io.Writer, topic []string) error {
	if len(topic) == 0 {
		entries := make([]commandDirectoryEntry, 0, len(commandCatalog()))
		for _, spec := range commandCatalog() {
			entries = append(entries, commandDirectoryEntry{Path: spec.Path, Summary: spec.Summary})
		}
		return writeResult(stdout, "help", map[string]any{"commands": entries, "globalFlags": globalFlags("")})
	}
	path := strings.Join(topic, " ")
	for _, spec := range commandCatalog() {
		if spec.Path == path {
			return writeResult(stdout, "help "+path, buildCommandHelp(spec))
		}
	}
	prefix := path + " "
	entries := make([]commandDirectoryEntry, 0)
	for _, spec := range commandCatalog() {
		if strings.HasPrefix(spec.Path, prefix) {
			entries = append(entries, commandDirectoryEntry{Path: spec.Path, Summary: spec.Summary})
		}
	}
	if len(entries) == 0 {
		return usagef("unknown help topic %q", path)
	}
	return writeResult(stdout, "help "+path, map[string]any{"commands": entries, "globalFlags": globalFlags(path)})
}

func buildCommandHelp(spec commandSpec) commandHelp {
	flags := commandFlags(spec)
	if flags == nil {
		flags = []helpFlag{}
	}
	return commandHelp{
		Path:          spec.Path,
		Summary:       spec.Summary,
		Usage:         kongUsage(strings.Fields(spec.Path)),
		Prerequisites: spec.Prerequisites,
		GlobalFlags:   globalFlags(spec.Path),
		Flags:         flags,
		Request:       requestMetadata(spec),
		ResultSchema:  resultSchema(spec.Result),
	}
}

func globalFlags(path string) []helpFlag {
	requiresSession := false
	for _, spec := range commandCatalog() {
		if spec.Path == path {
			requiresSession = true
			break
		}
	}
	flags := []helpFlag{
		{Name: "--session", Description: "Named local session. Required for all commands other than help.", Required: requiresSession},
		{Name: "--timeout", Description: "HTTP timeout. Defaults to 15s; wait commands use server wait plus 5s unless explicitly set."},
		{Name: "--insecure-skip-tls-verify, -K", Description: "Disable TLS certificate verification for this command."},
	}
	if path == "auth login" {
		flags = append([]helpFlag{{Name: "--api-base", Description: "HTTP(S) API origin or /agent/v1 base. Required for login.", Required: true}}, flags...)
	}
	return flags
}

func commandFlags(spec commandSpec) []helpFlag {
	switch spec.Path {
	case "auth login":
		return []helpFlag{{Name: "--token-stdin", Description: "Read exactly one token line from stdin.", Required: true, Sensitive: true}}
	case "challenge list":
		return []helpFlag{{Name: "--tag", Description: "Repeatable challenge tag filter."}, {Name: "--type", Description: "Optional challenge type filter."}}
	case "challenge get":
		return []helpFlag{{Name: "--challenge-id", Description: "Positive challenge ID.", Required: true}, {Name: "--page", Description: "Positive submit-log page number."}, {Name: "--size", Description: "Positive submit-log page size."}}
	case "attachment download":
		return []helpFlag{{Name: "--challenge-id", Description: "Positive challenge ID.", Required: true}, {Name: "--attachment-path", Description: "Exact attachment_path returned by challenge get.", Required: true}, {Name: "--output", Description: "New local output path; existing files are never replaced.", Required: true}}
	case "attachment dynamic status":
		return []helpFlag{{Name: "--challenge-id", Description: "Positive challenge ID.", Required: true}, {Name: "--regenerate", Description: "Request a new attachment; retries are not idempotent."}, {Name: "--wait-seconds", Description: "Server-side wait from 0 to 30 seconds."}, {Name: "--request-id", Description: spec.RequestID}}
	case "attachment dynamic download":
		return []helpFlag{{Name: "--challenge-id", Description: "Positive challenge ID.", Required: true}, {Name: "--output", Description: "New local output path; existing files are never replaced.", Required: true}}
	case "runtime start", "runtime stop", "runtime extend":
		return []helpFlag{{Name: "--challenge-id", Description: "Positive challenge ID.", Required: true}, {Name: "--request-id", Description: spec.RequestID, Required: true}}
	case "runtime inspect":
		return []helpFlag{{Name: "--challenge-id", Description: "Positive challenge ID.", Required: true}, {Name: "--wait-seconds", Description: "Server-side wait from 0 to 30 seconds."}}
	case "submission flag":
		return []helpFlag{{Name: "--challenge-id", Description: "Positive challenge ID.", Required: true}, {Name: "--request-id", Description: spec.RequestID, Required: true}, {Name: "--flag-stdin", Description: "Read exactly one candidate Flag line from stdin.", Required: true, Sensitive: true}}
	case "rank list":
		return []helpFlag{{Name: "--page", Description: "Positive page number."}, {Name: "--size", Description: "Positive page size."}}
	default:
		return nil
	}
}

func requestMetadata(spec commandSpec) map[string]any {
	if spec.Local {
		return map[string]any{"transport": "local", "operation": "delete named session"}
	}
	headers := []map[string]any{{"name": "Authorization", "source": "stored session token", "sensitive": true}}
	if spec.Path == "auth login" {
		headers = []map[string]any{{"name": "Authorization", "source": "--token-stdin", "sensitive": true}}
	}
	if spec.RequestID != "" {
		headers = append(headers, map[string]any{"name": "X-Request-Id", "source": "--request-id"})
	}
	metadata := map[string]any{
		"transport":    "http",
		"method":       spec.Endpoint.Method,
		"contractPath": spec.Endpoint.ContractPath,
		"bodySchema":   typeSchema(spec.Request),
		"headers":      headers,
	}
	if spec.Stdin != "" {
		metadata["stdin"] = map[string]any{"flag": spec.Stdin, "sensitive": true, "lines": 1}
	}
	return metadata
}

func kongUsage(path []string) string {
	schema := new(commandSchema)
	var output bytes.Buffer
	parser, err := kong.New(schema, kong.Name("ctfpc9n-cli"), kong.Writers(&output, io.Discard))
	if err != nil {
		return "ctfpc9n-cli " + strings.Join(path, " ")
	}
	context, err := kong.Trace(parser, path)
	if err != nil {
		return "ctfpc9n-cli " + strings.Join(path, " ")
	}
	if err := context.PrintUsage(false); err != nil {
		return "ctfpc9n-cli " + strings.Join(path, " ")
	}
	usage := output.String()
	globalUsage := "--session=NAME [--timeout=DURATION] [--insecure-skip-tls-verify]"
	if strings.Join(path, " ") == "auth login" {
		globalUsage = "--session=NAME --api-base=URL [--timeout=DURATION] [--insecure-skip-tls-verify]"
	}
	usage = strings.Replace(usage, "Usage: ctfpc9n-cli ", "Usage: ctfpc9n-cli "+globalUsage+" ", 1)
	return strings.TrimSpace(usage)
}

func resultSchema(data any) map[string]any {
	schema := typeSchema(data)
	definitions, hasDefinitions := schema["$defs"]
	delete(schema, "$defs")
	properties := map[string]any{
		"schemaVersion": map[string]any{"type": "string", "const": schemaVersion},
		"ok":            map[string]any{"type": "boolean"},
		"command":       map[string]any{"type": "string"},
		"data":          schema,
		"error": map[string]any{
			"type":     "object",
			"required": []string{"code", "message", "retryable"},
			"properties": map[string]any{
				"code":      map[string]any{"type": "string"},
				"message":   map[string]any{"type": "string"},
				"retryable": map[string]any{"type": "boolean"},
			},
		},
	}
	result := map[string]any{
		"$schema":    jsonSchemaDraft2020,
		"type":       "object",
		"required":   []string{"schemaVersion", "ok", "command"},
		"properties": properties,
	}
	if hasDefinitions {
		result["$defs"] = definitions
	}
	return result
}

func typeSchema(value any) map[string]any {
	definitions := make(map[string]any)
	typeOfValue := reflect.TypeOf(value)
	for typeOfValue != nil && typeOfValue.Kind() == reflect.Pointer {
		typeOfValue = typeOfValue.Elem()
	}
	schema := schemaForType(typeOfValue, definitions, make(map[reflect.Type]bool))
	if len(definitions) > 0 {
		schema["$defs"] = definitions
	}
	return schema
}

func schemaForType(value reflect.Type, definitions map[string]any, visiting map[reflect.Type]bool) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": schemaForType(value.Elem(), definitions, visiting)}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": schemaForType(value.Elem(), definitions, visiting)}
	case reflect.Interface:
		return map[string]any{}
	case reflect.Struct:
		key := strings.ReplaceAll(value.PkgPath(), "/", ".") + "." + value.Name()
		if value.Name() != "" {
			if _, exists := definitions[key]; exists || visiting[value] {
				return map[string]any{"$ref": "#/$defs/" + key}
			}
			visiting[value] = true
			definitions[key] = map[string]any{}
			body := structSchema(value, definitions, visiting)
			definitions[key] = body
			delete(visiting, value)
			return map[string]any{"$ref": "#/$defs/" + key}
		}
		return structSchema(value, definitions, visiting)
	default:
		return map[string]any{}
	}
}

func structSchema(value reflect.Type, definitions map[string]any, visiting map[reflect.Type]bool) map[string]any {
	properties := make(map[string]any)
	required := make([]string, 0)
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		if !field.IsExported() {
			continue
		}
		name, optional := jsonField(field)
		if name == "" {
			continue
		}
		properties[name] = schemaForType(field.Type, definitions, visiting)
		if !optional {
			required = append(required, name)
		}
	}
	sort.Strings(required)
	schema := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func jsonField(field reflect.StructField) (string, bool) {
	parts := strings.Split(field.Tag.Get("json"), ",")
	name := parts[0]
	if name == "" {
		name = field.Name
	}
	if name == "-" {
		return "", true
	}
	optional := false
	for _, option := range parts[1:] {
		if option == "optional" || option == "omitempty" {
			optional = true
		}
	}
	return name, optional
}
