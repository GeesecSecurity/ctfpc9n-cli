package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"ctfpc9n-cli/internal/agentapi"
	contract "ctfpc9n-cli/internal/generated/agentapi"
	"ctfpc9n-cli/internal/session"
)

const agentTokenEnvironment = "CTFPC9N_TOKEN"

func runCommand(ctx context.Context, options Options, schema *commandSchema, command string, stdin io.Reader, stdout io.Writer) error {
	stdin = stdinOrDefault(stdin)
	switch command {
	case "auth login":
		return runAuthLogin(ctx, options, schema.Auth.Login, stdin, stdout)
	case "auth logout":
		return runAuthLogout(options, stdout)
	}
	client, err := newClient(options)
	if err != nil {
		return err
	}
	switch command {
	case "stage progress":
		return runStageProgress(ctx, client, stdout)
	case "challenge list":
		return runChallengeList(ctx, client, schema.Challenge.List, stdout)
	case "challenge get":
		return runChallengeGet(ctx, client, schema.Challenge.Get, stdout)
	case "attachment download":
		return runAttachmentDownload(ctx, client, schema.Attachment.Download, stdout)
	case "attachment dynamic status":
		return runDynamicAttachmentStatus(ctx, client, schema.Attachment.Dynamic.Status, stdout)
	case "attachment dynamic download":
		return runDynamicAttachmentDownload(ctx, client, schema.Attachment.Dynamic.Download, stdout)
	case "runtime start":
		return runRuntimeWrite(ctx, client, "runtime start", contract.EndpointAgentStartRuntime, schema.Runtime.Start, stdout)
	case "runtime inspect":
		return runRuntimeInspect(ctx, client, schema.Runtime.Inspect, stdout)
	case "runtime stop":
		return runRuntimeWrite(ctx, client, "runtime stop", contract.EndpointAgentStopRuntime, schema.Runtime.Stop, stdout)
	case "runtime extend":
		return runRuntimeExtend(ctx, client, schema.Runtime.Extend, stdout)
	case "submission flag":
		return runFlagSubmission(ctx, client, schema.Submission.Flag, stdin, stdout)
	case "rank list":
		return runRankList(ctx, client, schema.Rank.List, stdout)
	default:
		return usagef("unsupported command %q", command)
	}
}

func runStageProgress(ctx context.Context, client *agentapi.Client, stdout io.Writer) error {
	var response contract.StageProgressResponse
	if err := client.Call(ctx, contract.EndpointAgentGetStageProgress, nil, &response, ""); err != nil {
		return err
	}
	return writeResult(stdout, "stage progress", response)
}

func runAuthLogin(ctx context.Context, options Options, command authLoginCommand, stdin io.Reader, stdout io.Writer) error {
	if err := validateSessionOption(options); err != nil {
		return err
	}
	if !options.APIBaseSet {
		return usagef("auth login requires --api-base")
	}
	token, err := resolveAuthToken(command, stdin)
	if err != nil {
		return err
	}
	baseURL, err := agentapi.NormalizeBase(options.APIBase)
	if err != nil {
		return usagef("%v", err)
	}
	client, err := agentapi.New(baseURL, strings.TrimSpace(token), options.Timeout, options.InsecureTLS)
	if err != nil {
		return usagef("%v", err)
	}
	if err := client.Call(ctx, contract.EndpointAgentListChallenges, &contract.AgentChallengeListRequest{}, nil, "", token); err != nil {
		return err
	}
	stored, err := session.Save(options.Session, session.Value{APIBase: baseURL, Token: token})
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	return writeResult(stdout, "auth login", authLoginResult{Session: options.Session, APIBase: stored.APIBase, CreatedAt: stored.CreatedAt.Format(time.RFC3339Nano)})
}

func resolveAuthToken(command authLoginCommand, stdin io.Reader) (string, error) {
	if command.Token != nil && command.TokenStdin {
		return "", usagef("--token and --token-stdin cannot be used together")
	}
	var token string
	switch {
	case command.Token != nil:
		token = *command.Token
	case command.TokenStdin:
		var err error
		token, err = readStdinLine(stdin, "token")
		if err != nil {
			return "", err
		}
	default:
		token = os.Getenv(agentTokenEnvironment)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", usagef("auth login requires --token, --token-stdin, or %s", agentTokenEnvironment)
	}
	return token, nil
}

func runAuthLogout(options Options, stdout io.Writer) error {
	if err := validateSessionOption(options); err != nil {
		return err
	}
	if options.APIBaseSet {
		return usagef("--api-base is only valid with auth login")
	}
	deleted, err := session.Delete(options.Session)
	if err != nil {
		return err
	}
	return writeResult(stdout, "auth logout", authLogoutResult{Deleted: deleted})
}

func runChallengeList(ctx context.Context, client *agentapi.Client, command challengeListCommand, stdout io.Writer) error {
	request := contract.AgentChallengeListRequest{Tags: command.Tags}
	if command.Type != nil {
		request.Type = *command.Type
	}
	var response contract.GetChallengeListResponse
	if err := client.Call(ctx, contract.EndpointAgentListChallenges, &request, &response, ""); err != nil {
		return err
	}
	return writeResult(stdout, "challenge list", response)
}

func runChallengeGet(ctx context.Context, client *agentapi.Client, command challengeGetCommand, stdout io.Writer) error {
	if err := isPositive(command.ChallengeID, "--challenge-id"); err != nil {
		return err
	}
	request := contract.AgentChallengeDetailRequest{ChallengeId: command.ChallengeID}
	var response contract.GetChallengeDetailInfoResponse
	if err := client.Call(ctx, contract.EndpointAgentGetChallenge, &request, &response, ""); err != nil {
		return err
	}
	return writeResult(stdout, "challenge get", response)
}

func runAttachmentDownload(ctx context.Context, client *agentapi.Client, command attachmentDownloadCommand, stdout io.Writer) error {
	if err := isPositive(command.ChallengeID, "--challenge-id"); err != nil {
		return err
	}
	if strings.TrimSpace(command.AttachmentPath) == "" {
		return usagef("--attachment-path must not be empty")
	}
	response, err := client.Download(ctx, contract.EndpointAgentDownloadAttachment, &contract.AgentAttachmentDownloadRequest{ChallengeId: command.ChallengeID, AttachmentPath: command.AttachmentPath}, command.Output)
	if err != nil {
		return err
	}
	return writeResult(stdout, "attachment download", response)
}

func runDynamicAttachmentStatus(ctx context.Context, client *agentapi.Client, command dynamicAttachmentStatusCommand, stdout io.Writer) error {
	if err := isPositive(command.ChallengeID, "--challenge-id"); err != nil {
		return err
	}
	if err := isWait(command.WaitSeconds); err != nil {
		return err
	}
	requestID := ""
	if command.Regenerate {
		var err error
		requestID, err = validateRequestID(command.RequestID)
		if err != nil {
			return err
		}
	} else if command.RequestID != "" {
		var err error
		requestID, err = validateRequestID(command.RequestID)
		if err != nil {
			return err
		}
	}
	request := contract.AgentDynamicAttachmentRequest{ChallengeId: command.ChallengeID, Regenerate: command.Regenerate, WaitSeconds: command.WaitSeconds}
	var response contract.AgentDynamicAttachmentStatusResponse
	if err := client.Call(ctx, contract.EndpointAgentGetDynamicAttachment, &request, &response, requestID); err != nil {
		return err
	}
	return writeResult(stdout, "attachment dynamic status", response)
}

func runDynamicAttachmentDownload(ctx context.Context, client *agentapi.Client, command dynamicAttachmentDownloadCommand, stdout io.Writer) error {
	if err := isPositive(command.ChallengeID, "--challenge-id"); err != nil {
		return err
	}
	response, err := client.Download(ctx, contract.EndpointAgentDownloadDynamicAttachment, &contract.AgentDynamicAttachmentDownloadRequest{ChallengeId: command.ChallengeID}, command.Output)
	if err != nil {
		return err
	}
	return writeResult(stdout, "attachment dynamic download", response)
}

func runRuntimeWrite(ctx context.Context, client *agentapi.Client, name string, endpoint contract.Endpoint, command runtimeWriteCommand, stdout io.Writer) error {
	if err := isPositive(command.ChallengeID, "--challenge-id"); err != nil {
		return err
	}
	requestID, err := validateRequestID(command.RequestID)
	if err != nil {
		return err
	}
	request := contract.AgentRuntimeRequest{ChallengeId: command.ChallengeID, RequestId: requestID}
	switch name {
	case "runtime start":
		var response contract.StartChallengeResponse
		if err := client.Call(ctx, endpoint, &request, &response, requestID); err != nil {
			return err
		}
		return writeResult(stdout, name, response)
	case "runtime stop":
		var response contract.BoolResp
		if err := client.Call(ctx, endpoint, &request, &response, requestID); err != nil {
			return err
		}
		return writeResult(stdout, name, response)
	default:
		return usagef("unsupported runtime write command %q", name)
	}
}

func runRuntimeInspect(ctx context.Context, client *agentapi.Client, command runtimeInspectCommand, stdout io.Writer) error {
	if err := isPositive(command.ChallengeID, "--challenge-id"); err != nil {
		return err
	}
	if err := isWait(command.WaitSeconds); err != nil {
		return err
	}
	request := contract.AgentRuntimeInspectRequest{ChallengeId: command.ChallengeID, WaitSeconds: command.WaitSeconds}
	var response contract.AgentRuntimeInspectResponse
	if err := client.Call(ctx, contract.EndpointAgentInspectRuntime, &request, &response, ""); err != nil {
		return err
	}
	return writeResult(stdout, "runtime inspect", response)
}

func runRuntimeExtend(ctx context.Context, client *agentapi.Client, command runtimeWriteCommand, stdout io.Writer) error {
	if err := isPositive(command.ChallengeID, "--challenge-id"); err != nil {
		return err
	}
	requestID, err := validateRequestID(command.RequestID)
	if err != nil {
		return err
	}
	request := contract.AgentRuntimeRequest{ChallengeId: command.ChallengeID, RequestId: requestID}
	var response contract.DelayChallengeResponse
	if err := client.Call(ctx, contract.EndpointAgentExtendRuntime, &request, &response, requestID); err != nil {
		return err
	}
	return writeResult(stdout, "runtime extend", response)
}

func runFlagSubmission(ctx context.Context, client *agentapi.Client, command submissionFlagCommand, stdin io.Reader, stdout io.Writer) error {
	if err := isPositive(command.ChallengeID, "--challenge-id"); err != nil {
		return err
	}
	requestID, err := validateRequestID(command.RequestID)
	if err != nil {
		return err
	}
	if !command.FlagStdin {
		return usagef("submission flag requires --flag-stdin")
	}
	flag, err := readStdinLine(stdin, "Flag")
	if err != nil {
		return err
	}
	request := contract.AgentFlagSubmitRequest{ChallengeId: command.ChallengeID, Flag: flag, RequestId: requestID}
	var response contract.BoolResp
	if err := client.Call(ctx, contract.EndpointAgentSubmitFlag, &request, &response, requestID, flag); err != nil {
		return err
	}
	return writeResult(stdout, "submission flag", response)
}

func runRankList(ctx context.Context, client *agentapi.Client, command rankListCommand, stdout io.Writer) error {
	if err := isPage(command.Page, "--page"); err != nil {
		return err
	}
	if err := isPage(command.Size, "--size"); err != nil {
		return err
	}
	request := contract.AgentRankRequest{Page: command.Page, Size: command.Size}
	var response contract.GetRankInfoResponse
	if err := client.Call(ctx, contract.EndpointAgentGetRank, &request, &response, ""); err != nil {
		return err
	}
	return writeResult(stdout, "rank list", response)
}
