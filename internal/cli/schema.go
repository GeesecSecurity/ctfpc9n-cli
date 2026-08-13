package cli

type commandSchema struct {
	Auth       authCommands       `cmd:""`
	Stage      stageCommands      `cmd:""`
	Challenge  challengeCommands  `cmd:""`
	Attachment attachmentCommands `cmd:""`
	Runtime    runtimeCommands    `cmd:""`
	Submission submissionCommands `cmd:""`
	Rank       rankCommands       `cmd:""`
}

type authCommands struct {
	Login  authLoginCommand  `cmd:""`
	Logout authLogoutCommand `cmd:""`
}

type authLoginCommand struct {
	TokenStdin bool `name:"token-stdin" required:"" help:"Read the Agent token from one stdin line."`
}

type authLogoutCommand struct{}

type stageCommands struct {
	Progress stageProgressCommand `cmd:""`
}

type stageProgressCommand struct{}

type challengeCommands struct {
	List challengeListCommand `cmd:""`
	Get  challengeGetCommand  `cmd:""`
}

type challengeListCommand struct {
	Tags []string `name:"tag" optional:"" help:"Repeatable challenge tag filter."`
	Type *uint64  `name:"type" optional:"" help:"Optional challenge type filter."`
}

type challengeGetCommand struct {
	ChallengeID uint64 `name:"challenge-id" required:"" help:"Positive challenge ID."`
	Page        uint64 `name:"page" default:"1" help:"Submit-log page number."`
	Size        uint64 `name:"size" default:"100" help:"Submit-log page size."`
}

type attachmentCommands struct {
	Download attachmentDownloadCommand `cmd:""`
	Dynamic  dynamicAttachmentCommands `cmd:""`
}

type attachmentDownloadCommand struct {
	ChallengeID    uint64 `name:"challenge-id" required:"" help:"Positive challenge ID."`
	AttachmentPath string `name:"attachment-path" required:"" help:"Exact attachment_path returned by challenge get."`
	Output         string `name:"output" required:"" help:"Explicit local output path."`
}

type dynamicAttachmentCommands struct {
	Status   dynamicAttachmentStatusCommand   `cmd:""`
	Download dynamicAttachmentDownloadCommand `cmd:""`
}

type dynamicAttachmentStatusCommand struct {
	ChallengeID uint64 `name:"challenge-id" required:"" help:"Positive challenge ID."`
	Regenerate  bool   `name:"regenerate" help:"Request a new dynamic attachment; retries are not idempotent."`
	WaitSeconds uint64 `name:"wait-seconds" default:"0" help:"Server-side wait duration from 0 to 30 seconds."`
	RequestID   string `name:"request-id" optional:"" help:"Audit request ID; required with --regenerate."`
}

type dynamicAttachmentDownloadCommand struct {
	ChallengeID uint64 `name:"challenge-id" required:"" help:"Positive challenge ID."`
	Output      string `name:"output" required:"" help:"Explicit local output path."`
}

type runtimeCommands struct {
	Start   runtimeWriteCommand   `cmd:""`
	Inspect runtimeInspectCommand `cmd:""`
	Stop    runtimeWriteCommand   `cmd:""`
	Extend  runtimeWriteCommand   `cmd:""`
}

type runtimeWriteCommand struct {
	ChallengeID uint64 `name:"challenge-id" required:"" help:"Positive challenge ID."`
	RequestID   string `name:"request-id" required:"" help:"Stable idempotency request ID."`
}

type runtimeInspectCommand struct {
	ChallengeID uint64 `name:"challenge-id" required:"" help:"Positive challenge ID."`
	WaitSeconds uint64 `name:"wait-seconds" default:"0" help:"Server-side wait duration from 0 to 30 seconds."`
}

type submissionCommands struct {
	Flag submissionFlagCommand `cmd:""`
}

type submissionFlagCommand struct {
	ChallengeID uint64 `name:"challenge-id" required:"" help:"Positive challenge ID."`
	RequestID   string `name:"request-id" required:"" help:"Stable idempotency request ID."`
	FlagStdin   bool   `name:"flag-stdin" required:"" help:"Read the candidate Flag from one stdin line."`
}

type rankCommands struct {
	List rankListCommand `cmd:""`
}

type rankListCommand struct {
	Page uint64 `name:"page" default:"1" help:"Page number."`
	Size uint64 `name:"size" default:"100" help:"Page size."`
}
