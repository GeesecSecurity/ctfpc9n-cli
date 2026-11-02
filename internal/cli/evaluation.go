package cli

import (
	"context"
	"ctfpc9n-cli/internal/agentapi"
	contract "ctfpc9n-cli/internal/generated/agentapi"
	evaluation "ctfpc9n-cli/internal/generated/evaluationapi"
	"ctfpc9n-cli/internal/session"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type evaluationLoginResult struct {
	Session string               `json:"session"`
	APIBase string               `json:"apiBase"`
	Kind    string               `json:"kind"`
	Account evaluation.AIAccount `json:"account"`
}
type evaluationCommands struct {
	Auth       evaluationAuth               `cmd:""`
	Runs       evaluationEmpty              `cmd:""`
	Run        evaluationRunCommands        `cmd:""`
	Challenge  evaluationChallengeCommands  `cmd:""`
	Attachment evaluationAttachmentCommands `cmd:""`
	Runtime    evaluationRuntimeCommands    `cmd:""`
	Submission evaluationSubmissionCommands `cmd:""`
	Patch      evaluationPatchCommands      `cmd:""`
	Report     evaluationRunQuery           `cmd:""`
}
type evaluationEmpty struct{}
type evaluationAuth struct {
	Login  evaluationLogin `cmd:""`
	Logout evaluationEmpty `cmd:""`
}
type evaluationLogin struct {
	TokenStdin bool `name:"token-stdin" required:"" help:"Read AI token from protected stdin only."`
}
type evaluationRunQuery struct {
	RunID    string `name:"run-id" required:""`
	Page     int    `name:"page" default:"1" help:"Page number, starting at 1."`
	PageSize int    `name:"page-size" default:"50" help:"Questions per page (1..100); read total to fetch subsequent pages."`
}
type evaluationRunWrite struct {
	RunID     string `name:"run-id" required:""`
	RequestID string `name:"request-id" required:""`
}
type evaluationRunCommands struct {
	Start  evaluationRunWrite `cmd:""`
	Finish evaluationRunWrite `cmd:""`
}
type evaluationChallengeQuery struct {
	RunID       string `name:"run-id" required:""`
	ChallengeID string `name:"challenge-id" required:""`
}
type evaluationChallengeCommands struct {
	List evaluationRunQuery       `cmd:""`
	Get  evaluationChallengeQuery `cmd:""`
}
type evaluationRuntimeWrite struct {
	RunID       string `name:"run-id" required:""`
	ChallengeID string `name:"challenge-id" required:""`
	RequestID   string `name:"request-id" required:""`
}
type evaluationRuntimeCommands struct {
	Start   evaluationRuntimeWrite   `cmd:""`
	Stop    evaluationRuntimeWrite   `cmd:""`
	Extend  evaluationRuntimeWrite   `cmd:""`
	Inspect evaluationChallengeQuery `cmd:""`
}
type evaluationFlag struct {
	evaluationRuntimeWrite
	FlagStdin bool `name:"flag-stdin" required:""`
}
type evaluationSubmissionCommands struct {
	Flag evaluationFlag `cmd:""`
}
type evaluationDownload struct {
	evaluationChallengeQuery
	AttachmentID string `name:"attachment-id" required:""`
	Output       string `name:"output" required:""`
}
type evaluationAttachmentCommands struct {
	Download evaluationDownload `cmd:""`
}
type evaluationPatchUpload struct {
	evaluationRuntimeWrite
	File string `name:"file" required:""`
}
type evaluationPatchSubmit struct {
	evaluationRuntimeWrite
	PatchID string `name:"patch-id" required:""`
}
type evaluationPatchCommands struct {
	Upload evaluationPatchUpload    `cmd:""`
	Submit evaluationPatchSubmit    `cmd:""`
	Status evaluationChallengeQuery `cmd:""`
}

func evalEndpoint(e evaluation.Endpoint) contract.Endpoint {
	return contract.Endpoint{Name: e.Name, Method: e.Method, Path: e.Path, ContractPath: e.ContractPath}
}

var evalIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)

func evaluationClient(options Options) (*agentapi.Client, error) {
	if e := validateSessionOption(options); e != nil {
		return nil, e
	}
	if options.APIBaseSet {
		return nil, usagef("--api-base only valid for evaluation auth login")
	}
	stored, e := session.Load(options.Session)
	if e != nil {
		return nil, fmt.Errorf("%w: %v", errAuth, e)
	}
	if stored.Kind != "evaluation" {
		return nil, usagef("competition session cannot be used for evaluation commands")
	}
	return agentapi.NewEvaluation(stored.APIBase, stored.Token, options.Timeout, options.InsecureTLS)
}
func runEvaluation(ctx context.Context, options Options, schema evaluationCommands, command string, stdin io.Reader, stdout io.Writer) error {
	if command == "evaluation auth logout" {
		if e := validateSessionOption(options); e != nil {
			return e
		}
		v, e := session.Load(options.Session)
		if e != nil {
			return fmt.Errorf("%w: %v", errAuth, e)
		}
		if v.Kind != "evaluation" {
			return usagef("cannot delete competition session via evaluation logout")
		}
		deleted, e := session.Delete(options.Session)
		if e != nil {
			return e
		}
		return writeResult(stdout, command, authLogoutResult{deleted})
	}
	if command == "evaluation auth login" {
		if e := validateSessionOption(options); e != nil {
			return e
		}
		if !schema.Auth.Login.TokenStdin || !options.APIBaseSet {
			return usagef("evaluation auth login requires --api-base and --token-stdin")
		}
		token, e := readStdinLine(io.LimitReader(stdin, 1025), "AI token")
		if e != nil {
			return e
		}
		if len(token) > 1024 {
			return usagef("AI token too long")
		}
		base, e := agentapi.NormalizeEvaluationBase(options.APIBase)
		if e != nil {
			return usagef("%v", e)
		}
		client, e := agentapi.NewEvaluation(base, token, options.Timeout, options.InsecureTLS)
		if e != nil {
			return e
		}
		var response evaluation.AISession
		if e = client.Call(ctx, evalEndpoint(evaluation.EndpointAiAgentSession), nil, &response, "", token); e != nil {
			return e
		}
		stored, e := session.Save(options.Session, session.Value{Kind: "evaluation", APIBase: base, Token: token})
		if e != nil {
			return e
		}
		return writeResult(stdout, command, evaluationLoginResult{Session: options.Session, APIBase: base, Kind: stored.Kind, Account: response.Account})
	}
	client, e := evaluationClient(options)
	if e != nil {
		return e
	}
	var endpoint evaluation.Endpoint
	var request any
	var secret string
	var requestID string
	q := evaluation.AIQuery{}
	w := evaluation.AIWrite{}
	setQuery := func(v evaluationChallengeQuery) { q.RunId = v.RunID; q.ChallengeId = v.ChallengeID }
	setWrite := func(v evaluationRuntimeWrite) {
		w.RunId = v.RunID
		w.ChallengeId = v.ChallengeID
		w.RequestId = v.RequestID
	}
	switch command {
	case "evaluation runs":
		endpoint = evaluation.EndpointAiAgentRuns
	case "evaluation run start":
		endpoint = evaluation.EndpointAiAgentStart
		w.RunId = schema.Run.Start.RunID
		w.RequestId = schema.Run.Start.RequestID
	case "evaluation run finish":
		endpoint = evaluation.EndpointAiAgentFinish
		w.RunId = schema.Run.Finish.RunID
		w.RequestId = schema.Run.Finish.RequestID
	case "evaluation challenge list":
		endpoint = evaluation.EndpointAiAgentChallenges
		q.RunId = schema.Challenge.List.RunID
		q.Page = schema.Challenge.List.Page
		q.PageSize = schema.Challenge.List.PageSize
	case "evaluation challenge get":
		endpoint = evaluation.EndpointAiAgentChallenge
		setQuery(schema.Challenge.Get)
	case "evaluation report":
		endpoint = evaluation.EndpointAiAgentReport
		q.RunId = schema.Report.RunID
		q.Page = schema.Report.Page
		q.PageSize = schema.Report.PageSize
	case "evaluation runtime inspect":
		endpoint = evaluation.EndpointAiAgentRuntime
		setQuery(schema.Runtime.Inspect)
	case "evaluation runtime start":
		endpoint = evaluation.EndpointAiAgentRuntimeStart
		setWrite(schema.Runtime.Start)
	case "evaluation runtime stop":
		endpoint = evaluation.EndpointAiAgentRuntimeStop
		setWrite(schema.Runtime.Stop)
	case "evaluation runtime extend":
		endpoint = evaluation.EndpointAiAgentRuntimeExtend
		setWrite(schema.Runtime.Extend)
	case "evaluation submission flag":
		endpoint = evaluation.EndpointAiAgentFlag
		setWrite(schema.Submission.Flag.evaluationRuntimeWrite)
		if !schema.Submission.Flag.FlagStdin {
			return usagef("--flag-stdin required")
		}
		secret, e = readStdinLine(io.LimitReader(stdin, 4097), "Flag")
		if e != nil {
			return e
		}
		if len(secret) > 4096 {
			return usagef("Flag exceeds limit")
		}
		w.Flag = secret
	case "evaluation patch status":
		endpoint = evaluation.EndpointAiAgentPatchStatus
		setQuery(schema.Patch.Status)
	case "evaluation patch submit":
		endpoint = evaluation.EndpointAiAgentPatchSubmit
		setWrite(schema.Patch.Submit.evaluationRuntimeWrite)
		w.PatchId = schema.Patch.Submit.PatchID
	case "evaluation patch upload":
		endpoint = evaluation.EndpointAiAgentPatchUpload
		setWrite(schema.Patch.Upload.evaluationRuntimeWrite)
		f, e := os.Open(schema.Patch.Upload.File)
		if e != nil {
			return usagef("cannot open patch file")
		}
		defer f.Close()
		data, e := io.ReadAll(io.LimitReader(f, 10*1024*1024+1))
		if e != nil || len(data) == 0 || len(data) > 10*1024*1024 {
			return usagef("patch must be 1..10MiB")
		}
		w.FileName = filepath.Base(schema.Patch.Upload.File)
		w.ContentBase64 = base64.StdEncoding.EncodeToString(data)
	case "evaluation attachment download":
		endpoint = evaluation.EndpointAiDownloadAttachment
		setQuery(schema.Attachment.Download.evaluationChallengeQuery)
		q.AttachmentId = schema.Attachment.Download.AttachmentID
	default:
		return usagef("unsupported evaluation command")
	}
	if endpoint.Method == "POST" {
		if !evalIDPattern.MatchString(w.RunId) || !evalIDPattern.MatchString(w.RequestId) || len(w.RequestId) < 8 {
			return usagef("valid --run-id and stable --request-id (8..128 chars) required")
		}
		request = &w
		requestID = w.RequestId
	} else if command != "evaluation runs" {
		if !evalIDPattern.MatchString(q.RunId) {
			return usagef("invalid --run-id")
		}
		request = &q
	}
	if command == "evaluation attachment download" {
		result, e := client.Download(ctx, evalEndpoint(endpoint), request, schema.Attachment.Download.Output)
		if e != nil {
			return e
		}
		return writeResult(stdout, command, result)
	}
	if (command == "evaluation challenge list" || command == "evaluation report") && (q.Page < 1 || q.PageSize < 1 || q.PageSize > 100) {
		return usagef("--page must be positive and --page-size must be 1..100")
	}
	var result json.RawMessage
	if e = client.Call(ctx, evalEndpoint(endpoint), request, &result, requestID, secret); e != nil {
		return e
	}
	return writeResult(stdout, command, result)
}
func evaluationCommandCatalog() []commandSpec {
	endpoints := map[string]evaluation.Endpoint{"evaluation runs": evaluation.EndpointAiAgentRuns, "evaluation run start": evaluation.EndpointAiAgentStart, "evaluation run finish": evaluation.EndpointAiAgentFinish, "evaluation challenge list": evaluation.EndpointAiAgentChallenges, "evaluation challenge get": evaluation.EndpointAiAgentChallenge, "evaluation attachment download": evaluation.EndpointAiDownloadAttachment, "evaluation runtime start": evaluation.EndpointAiAgentRuntimeStart, "evaluation runtime inspect": evaluation.EndpointAiAgentRuntime, "evaluation runtime stop": evaluation.EndpointAiAgentRuntimeStop, "evaluation runtime extend": evaluation.EndpointAiAgentRuntimeExtend, "evaluation submission flag": evaluation.EndpointAiAgentFlag, "evaluation patch upload": evaluation.EndpointAiAgentPatchUpload, "evaluation patch submit": evaluation.EndpointAiAgentPatchSubmit, "evaluation patch status": evaluation.EndpointAiAgentPatchStatus, "evaluation report": evaluation.EndpointAiAgentReport}
	loginEndpoint := evalEndpoint(evaluation.EndpointAiAgentSession)
	out := []commandSpec{{Endpoint: &loginEndpoint, Path: "evaluation auth login", Summary: "Validate AI token from stdin and save an evaluation-only session.", Stdin: "--token-stdin", Result: &evaluationLoginResult{}}, {Path: "evaluation auth logout", Summary: "Delete evaluation-only session.", Local: true, Result: &authLogoutResult{}}}
	results := map[string]any{
		"evaluation runs":      &evaluation.AIRunList{},
		"evaluation run start": &evaluation.AIRun{}, "evaluation run finish": &evaluation.AIRun{},
		"evaluation challenge list": &evaluation.AIChallengeList{}, "evaluation challenge get": &evaluation.AIChallenge{},
		"evaluation attachment download": &agentapi.DownloadResult{},
		"evaluation runtime start":       &evaluation.AIRuntime{}, "evaluation runtime inspect": &evaluation.AIRuntime{}, "evaluation runtime stop": &evaluation.AIRuntime{}, "evaluation runtime extend": &evaluation.AIRuntime{},
		"evaluation submission flag": &evaluation.AIFlagResult{},
		"evaluation patch upload":    &evaluation.AIPatchResult{}, "evaluation patch submit": &evaluation.AIJob{}, "evaluation patch status": &evaluation.AIJob{},
		"evaluation report": &evaluation.AIReport{},
	}
	for path, e := range endpoints {
		converted := evalEndpoint(e)
		prerequisites := []string{"An evaluation-only session; IDs are opaque strings."}
		if path == "evaluation run start" {
			prerequisites = append(prerequisites, "Only start a created run. Completed or timed_out runs are read-only by default; resume one only when the user explicitly requests it. A resume opens a new period for new questions and questions that require retesting.")
		}
		if path == "evaluation challenge list" {
			prerequisites = append(prerequisites, "If a result item has locked: true, skip it: do not submit a Flag, start or extend its runtime, or submit a patch.")
		}
		spec := commandSpec{Path: path, Summary: "Operate only the authenticated AI identity's evaluation resources.", Endpoint: &converted, Request: &evaluation.AIQuery{}, Result: results[path], Prerequisites: prerequisites}
		if path == "evaluation runs" {
			spec.Request = nil
		}
		if e.Method == "POST" {
			spec.Request = &evaluation.AIWrite{}
			spec.RequestID = "Stable idempotency request ID; preserve on retry."
		}
		if strings.HasSuffix(path, "submission flag") {
			spec.Stdin = "--flag-stdin"
			spec.Result = &evaluation.AIFlagResult{}
		}
		out = append(out, spec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
func evaluationHelpFlags(path string) []helpFlag {
	if path == "evaluation auth login" {
		return []helpFlag{{Name: "--token-stdin", Description: "AI token from protected stdin only", Required: true, Sensitive: true}}
	}
	if path == "evaluation auth logout" || path == "evaluation runs" {
		return []helpFlag{}
	}
	out := []helpFlag{{Name: "--run-id", Description: "Opaque evaluation run ID", Required: true}}
	if path == "evaluation challenge list" || path == "evaluation report" {
		out = append(out, helpFlag{Name: "--page", Description: "Page number (default 1)"}, helpFlag{Name: "--page-size", Description: "Questions per page (1..100, default 50); inspect total for more pages"})
	}
	if strings.Contains(path, "runtime") || strings.Contains(path, "patch") || strings.Contains(path, "challenge get") || strings.Contains(path, "attachment") || strings.Contains(path, "submission") {
		out = append(out, helpFlag{Name: "--challenge-id", Description: "Opaque challenge ID", Required: true})
	}
	if strings.HasSuffix(path, "start") || strings.HasSuffix(path, "finish") || strings.HasSuffix(path, "stop") || strings.HasSuffix(path, "extend") || strings.Contains(path, "submission") || strings.HasSuffix(path, "upload") || strings.HasSuffix(path, "submit") {
		out = append(out, helpFlag{Name: "--request-id", Description: "Stable idempotency key, 8..128 characters", Required: true})
	}
	if strings.Contains(path, "submission") {
		out = append(out, helpFlag{Name: "--flag-stdin", Description: "Read Flag from stdin only", Required: true, Sensitive: true})
	}
	if strings.HasSuffix(path, "upload") {
		out = append(out, helpFlag{Name: "--file", Description: "Local ZIP/tar patch, at most 10MiB", Required: true})
	}
	if strings.HasSuffix(path, "submit") {
		out = append(out, helpFlag{Name: "--patch-id", Description: "Server-issued uploaded patch ID", Required: true})
	}
	if strings.Contains(path, "attachment") {
		out = append(out, helpFlag{Name: "--attachment-id", Description: "Declared attachment ID", Required: true}, helpFlag{Name: "--output", Description: "New local output file; never overwrite", Required: true})
	}
	return out
}
