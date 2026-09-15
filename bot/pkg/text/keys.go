package text

// Every constant here is one sentence; english.go and german.go hold the words.
// Each is written out with its type, which is what lets the tests find them all.

// Answers to any /au command.
const (
	NotInServer        Key = "not-in-server"
	StorageUnavailable Key = "storage-unavailable"
	InvalidCommand     Key = "invalid-command"
	ServerUnavailable  Key = "server-unavailable"
	NotAuthorized      Key = "not-authorized"
	CommandFailed      Key = "command-failed"
)

// /au setup, /au settings, /au link and /au unlink.
const (
	ChannelsSaved    Key = "channels-saved"
	ChannelList      Key = "channel-list"
	NotConfigured    Key = "not-configured"
	AdminsSet        Key = "admins-set"
	ConfigReset      Key = "config-reset"
	SettingsSaved    Key = "settings-saved"
	SettingsList     Key = "settings-list"
	Yes              Key = "yes"
	No               Key = "no"
	Linked           Key = "linked"
	Unlinked         Key = "unlinked"
	FallbackDatabase Key = "fallback-database"
	FallbackReady    Key = "fallback-ready"
	FallbackNeedsBot Key = "fallback-needs-bot"
)

// What makes a configuration invalid or not ready.
const (
	ProblemMissing       Key = "problem-missing"
	ProblemOneOf         Key = "problem-one-of"
	ProblemTimeoutBounds Key = "problem-timeout-bounds"
	ProblemSameChannel   Key = "problem-same-channel"
	ProblemDisabled      Key = "problem-disabled"
	ProblemNotSet        Key = "problem-not-set"
)

// /au capture.
const (
	PairingCode               Key = "pairing-code"
	NothingToRevoke           Key = "nothing-to-revoke"
	Revoked                   Key = "revoked"
	CaptureUnavailable        Key = "capture-unavailable"
	CaptureNeverConnected     Key = "capture-never-connected"
	CapturePaired             Key = "capture-paired"
	CaptureNotPaired          Key = "capture-not-paired"
	CaptureCodeWaiting        Key = "capture-code-waiting"
	CaptureRevokedCredentials Key = "capture-revoked-credentials"
	CaptureInstalls           Key = "capture-installs"
)

// /au session.
const (
	SessionUnavailable Key = "session-unavailable"
	NotSetUp           Key = "not-set-up"
	AlreadyManaging    Key = "already-managing"
	NowManaging        Key = "now-managing"
	NotManaging        Key = "not-managing"
	StoppedManaging    Key = "stopped-managing"
	AlreadyPaused      Key = "already-paused"
	NothingToPause     Key = "nothing-to-pause"
	PausedNow          Key = "paused-now"
	Resumed            Key = "resumed"
	StatusManaging     Key = "status-managing"
	StatusPaused       Key = "status-paused"
	StatusStopped      Key = "status-stopped"
	StatusNoGame       Key = "status-no-game"
	StatusPhase        Key = "status-phase"
	StatusPlayers      Key = "status-players"
	StatusNoChannels   Key = "status-no-channels"
)

// Game phases and session modes, as a status names them.
const (
	PhaseMenu          Key = "phase-menu"
	PhaseLobby         Key = "phase-lobby"
	PhaseTasks         Key = "phase-tasks"
	PhaseDiscussion    Key = "phase-discussion"
	PhaseBetweenRounds Key = "phase-between-rounds"
	PhaseUnknown       Key = "phase-unknown"
	ModeRunning        Key = "mode-running"
	ModePaused         Key = "mode-paused"
	ModeStopped        Key = "mode-stopped"
)

// Private answers when choosing a crewmate.
const (
	CrewmateMenuFailed   Key = "crewmate-menu-failed"
	NoLobbyYet           Key = "no-lobby-yet"
	CrewmateOnlyInServer Key = "crewmate-only-in-server"
	ChoiceNotUnderstood  Key = "choice-not-understood"
	PlayerLeftLobby      Key = "player-left-lobby"
	ChoiceNotAllowed     Key = "choice-not-allowed"
	ChoiceNotSaved       Key = "choice-not-saved"
)

// /au doctor: what each check is called.
const (
	CheckDiscord        Key = "check-discord"
	CheckDatabase       Key = "check-database"
	CheckConfiguration  Key = "check-configuration"
	CheckEnabled        Key = "check-enabled"
	CheckMainChannel    Key = "check-main-channel"
	CheckGhostChannel   Key = "check-ghost-channel"
	CheckControlChannel Key = "check-control-channel"
	CheckPermissions    Key = "check-permissions"
	CheckCrewmateMenu   Key = "check-crewmate-menu"
	CheckCapture        Key = "check-capture"
	CheckHeartbeat      Key = "check-heartbeat"
	CheckProtocol       Key = "check-protocol"
	CheckGameState      Key = "check-game-state"
	CheckBuild          Key = "check-build"
)

// /au doctor: what the checks found and what to do about it.
const (
	DoctorNothingChecked         Key = "doctor-nothing-checked"
	DoctorProblemsAndWarnings    Key = "doctor-problems-and-warnings"
	DoctorProblems               Key = "doctor-problems"
	DoctorWarnings               Key = "doctor-warnings"
	DoctorAllGood                Key = "doctor-all-good"
	DoctorNoSession              Key = "doctor-no-session"
	DoctorNoSessionFix           Key = "doctor-no-session-fix"
	DoctorGuildNotCached         Key = "doctor-guild-not-cached"
	DoctorGuildNotCachedFix      Key = "doctor-guild-not-cached-fix"
	DoctorConnected              Key = "doctor-connected"
	DoctorDatabaseUnreachable    Key = "doctor-database-unreachable"
	DoctorDatabaseUnreachableFix Key = "doctor-database-unreachable-fix"
	DoctorDatabaseReachable      Key = "doctor-database-reachable"
	DoctorConfigUnreadable       Key = "doctor-config-unreadable"
	DoctorConfigInvalidFix       Key = "doctor-config-invalid-fix"
	DoctorDisabled               Key = "doctor-disabled"
	DoctorDisabledFix            Key = "doctor-disabled-fix"
	DoctorNotSet                 Key = "doctor-not-set"
	DoctorGhostNotUsed           Key = "doctor-ghost-not-used"
	DoctorSetChannelFix          Key = "doctor-set-channel-fix"
	DoctorControlChannelFix      Key = "doctor-control-channel-fix"
	DoctorChannelUnconfirmed     Key = "doctor-channel-unconfirmed"
	DoctorChannelMissing         Key = "doctor-channel-missing"
	DoctorChannelMissingFix      Key = "doctor-channel-missing-fix"
	DoctorPermissionsGranted     Key = "doctor-permissions-granted"
	DoctorPermissionsFix         Key = "doctor-permissions-fix"
	DoctorCrewmatePermissionsFix Key = "doctor-crewmate-permissions-fix"
	DoctorCrewmateUnavailable    Key = "doctor-crewmate-unavailable"
	DoctorCrewmateUnavailableFix Key = "doctor-crewmate-unavailable-fix"
	DoctorCrewmatePictures       Key = "doctor-crewmate-pictures"
	DoctorCrewmatePicturesFix    Key = "doctor-crewmate-pictures-fix"
	DoctorCrewmateReady          Key = "doctor-crewmate-ready"
	DoctorCaptureUnreadable      Key = "doctor-capture-unreadable"
	DoctorCaptureNotPaired       Key = "doctor-capture-not-paired"
	DoctorCaptureNotPairedFix    Key = "doctor-capture-not-paired-fix"
	DoctorCapturePaired          Key = "doctor-capture-paired"
	DoctorHeartbeatNever         Key = "doctor-heartbeat-never"
	DoctorHeartbeatNeverFix      Key = "doctor-heartbeat-never-fix"
	DoctorHeartbeatStale         Key = "doctor-heartbeat-stale"
	DoctorHeartbeatStaleFix      Key = "doctor-heartbeat-stale-fix"
	DoctorHeartbeatRecent        Key = "doctor-heartbeat-recent"
	DoctorProtocol               Key = "doctor-protocol"
	DoctorNoGameData             Key = "doctor-no-game-data"
	DoctorNoGameDataFix          Key = "doctor-no-game-data-fix"
	DoctorGameState              Key = "doctor-game-state"
	DoctorStartSessionFix        Key = "doctor-start-session-fix"
)

// Missing Discord permissions. The permission names themselves stay Discord's
// English ones: they are what the API documents, and a guessed translation of a
// role setting is worse than none.
const (
	PermissionsMissing      Key = "permissions-missing"
	PermissionMissingLine   Key = "permission-missing-line"
	PermissionChannelsJoin  Key = "permission-channels-join"
	PurposeMainChannel      Key = "purpose-main-channel"
	PurposeGhostChannel     Key = "purpose-ghost-channel"
	PurposeTextChannel      Key = "purpose-text-channel"
	WithoutViewVoiceChannel Key = "without-view-voice-channel"
	WithoutConnect          Key = "without-connect"
	WithoutMoveMembers      Key = "without-move-members"
	WithoutMuteMembers      Key = "without-mute-members"
	WithoutDeafenMembers    Key = "without-deafen-members"
	WithoutViewTextChannel  Key = "without-view-text-channel"
	WithoutSendMessages     Key = "without-send-messages"
	WithoutEmbedLinks       Key = "without-embed-links"
)

// Descriptions of the /au commands and their options, as Discord shows them
// while typing. Discord allows at most 100 characters each.
const (
	DescribeAU               Key = "describe-au"
	DescribeSetup            Key = "describe-setup"
	DescribeSetupChannels    Key = "describe-setup-channels"
	DescribeMainChannel      Key = "describe-main-channel"
	DescribeGhostChannel     Key = "describe-ghost-channel"
	DescribeControlChannel   Key = "describe-control-channel"
	DescribeSetupPermissions Key = "describe-setup-permissions"
	DescribeAdminRole        Key = "describe-admin-role"
	DescribeSetupReset       Key = "describe-setup-reset"
	DescribeConfirmReset     Key = "describe-confirm-reset"
	DescribeSettings         Key = "describe-settings"
	DescribeSettingsShow     Key = "describe-settings-show"
	DescribeSettingsPreset   Key = "describe-settings-preset"
	DescribePolicyPreset     Key = "describe-policy-preset"
	DescribeSettingsVoice    Key = "describe-settings-voice"
	DescribeEnabled          Key = "describe-enabled"
	DescribePolicy           Key = "describe-policy"
	DescribeSettingsGhosts   Key = "describe-settings-ghosts"
	DescribeAutoMoveGhosts   Key = "describe-auto-move-ghosts"
	DescribeEnforce          Key = "describe-enforce"
	DescribeSettingsSafety   Key = "describe-settings-safety"
	DescribeTimeout          Key = "describe-timeout"
	DescribeTimeoutAction    Key = "describe-timeout-action"
	DescribeAutoStart        Key = "describe-auto-start"
	DescribeSettingsExport   Key = "describe-settings-export"
	DescribeCapture          Key = "describe-capture"
	DescribeCapturePair      Key = "describe-capture-pair"
	DescribeCaptureStatus    Key = "describe-capture-status"
	DescribeCaptureRevoke    Key = "describe-capture-revoke"
	DescribeConfirmRevoke    Key = "describe-confirm-revoke"
	DescribeSession          Key = "describe-session"
	DescribeSessionStart     Key = "describe-session-start"
	DescribeSessionStop      Key = "describe-session-stop"
	DescribeSessionPause     Key = "describe-session-pause"
	DescribeSessionResume    Key = "describe-session-resume"
	DescribeSessionStatus    Key = "describe-session-status"
	DescribeLink             Key = "describe-link"
	DescribePlayer           Key = "describe-player"
	DescribeUser             Key = "describe-user"
	DescribeUnlink           Key = "describe-unlink"
	DescribeDoctor           Key = "describe-doctor"
	DescribeVersion          Key = "describe-version"
)

// /au settings language.
const (
	LanguageSameAsDiscord    Key = "language-same-as-discord"
	DescribeSettingsLanguage Key = "describe-settings-language"
	DescribeLanguage         Key = "describe-language"
)

// The crewmate message, which everybody in the text channel reads.
const (
	BoardTitle       Key = "board-title"
	BoardWaiting     Key = "board-waiting"
	BoardIntro       Key = "board-intro"
	BoardFree        Key = "board-free"
	BoardTaken       Key = "board-taken"
	BoardFooter      Key = "board-footer"
	BoardUnlink      Key = "board-unlink"
	BoardUnlinkHint  Key = "board-unlink-hint"
	BoardPlaceholder Key = "board-placeholder"
)

// Crewmate colours, in the order the game numbers them.
const (
	ColorRed     Key = "color-red"
	ColorBlue    Key = "color-blue"
	ColorGreen   Key = "color-green"
	ColorPink    Key = "color-pink"
	ColorOrange  Key = "color-orange"
	ColorYellow  Key = "color-yellow"
	ColorBlack   Key = "color-black"
	ColorWhite   Key = "color-white"
	ColorPurple  Key = "color-purple"
	ColorBrown   Key = "color-brown"
	ColorCyan    Key = "color-cyan"
	ColorLime    Key = "color-lime"
	ColorMaroon  Key = "color-maroon"
	ColorRose    Key = "color-rose"
	ColorBanana  Key = "color-banana"
	ColorGray    Key = "color-gray"
	ColorTan     Key = "color-tan"
	ColorCoral   Key = "color-coral"
	ColorUnknown Key = "color-unknown"
)

// Notices in the text channel.
const (
	NoticeCaptureStopped  Key = "notice-capture-stopped"
	NoticePausedByChoice  Key = "notice-paused-by-choice"
	NoticeReleaseFailed   Key = "notice-release-failed"
	NoticeReleased        Key = "notice-released"
	NoticeResumesOnItsOwn Key = "notice-resumes-on-its-own"
	NoticeCaptureBack     Key = "notice-capture-back"
)

// The round on the crewmate message: map, phase and lobby code.
const (
	BoardMap   Key = "board-map"
	BoardPhase Key = "board-phase"
	BoardCode  Key = "board-code"
)
