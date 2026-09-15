package bot

import (
	"fmt"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/doctor"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/permission"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
)

// Doctor gathers the facts /au doctor reports on.
//
// The facts are gathered here, where the connections are, and judged in
// pkg/doctor, where nothing can fail. Nothing gathered is a secret: the checks
// describe whether something works, never what it was configured with, so the
// report cannot leak a credential into a Discord channel.
type Doctor struct {
	bot *Bot
}

// NewDoctor wires the diagnosis to a bot.
func NewDoctor(bot *Bot) *Doctor { return &Doctor{bot: bot} }

// Diagnose runs every check for one guild and renders them for Discord, in the
// language of whoever asked.
func (d *Doctor) Diagnose(guildID string, language text.Language) (string, error) {
	return d.Report(guildID, language).Render(language), nil
}

// Report runs every check for one guild and returns them unrendered, for the
// Windows app, which shows them in its own window rather than a message.
func (d *Doctor) Report(guildID string, language text.Language) doctor.Report {
	var report doctor.Report

	d.checkDiscord(&report, guildID, language)
	config, configured := d.checkStorage(&report, guildID, language)
	d.checkChannels(&report, config, configured, language)
	d.checkPermissions(&report, config, configured, language)
	d.checkCrewmates(&report, config, configured, language)
	d.checkCapture(&report, guildID, language)
	d.checkSession(&report, guildID, language)
	d.checkBuild(&report, language)

	return report
}

// checkDiscord reports whether the bot is connected and can see the guild.
func (d *Doctor) checkDiscord(report *doctor.Report, guildID string, language text.Language) {
	name := language.Say(text.CheckDiscord)

	if d.bot.PrimarySession == nil {
		report.Add(doctor.Check{
			Name: name, Level: doctor.Fail,
			Detail: language.Say(text.DoctorNoSession),
			Fix:    language.Say(text.DoctorNoSessionFix),
		})
		return
	}

	guild, err := d.bot.PrimarySession.State.Guild(guildID)
	if err != nil || guild == nil {
		report.Add(doctor.Check{
			Name: name, Level: doctor.Fail,
			Detail: language.Say(text.DoctorGuildNotCached),
			Fix:    language.Say(text.DoctorGuildNotCachedFix),
		})
		return
	}

	report.Add(doctor.Check{
		Name: name, Level: doctor.OK,
		Detail: language.Say(text.DoctorConnected, guild.Name),
	})
}

// checkStorage reports on SQLite and the guild's configuration, and hands back
// what the later checks need.
func (d *Doctor) checkStorage(report *doctor.Report, guildID string, language text.Language) (voiceChannels, bool) {
	version, err := d.bot.AUVC.SchemaVersion()
	if err != nil {
		report.Add(doctor.Check{
			Name: language.Say(text.CheckDatabase), Level: doctor.Fail,
			Detail: language.Say(text.DoctorDatabaseUnreachable, err.Error()),
			Fix:    language.Say(text.DoctorDatabaseUnreachableFix),
		})
		return voiceChannels{}, false
	}

	report.Add(doctor.Check{
		Name: language.Say(text.CheckDatabase), Level: doctor.OK,
		Detail: language.Say(text.DoctorDatabaseReachable, version),
	})

	config, err := d.bot.AUVC.GuildConfig(guildID)
	if err != nil {
		report.Add(doctor.Check{
			Name: language.Say(text.CheckConfiguration), Level: doctor.Fail,
			Detail: language.Say(text.DoctorConfigUnreadable, err.Error()),
		})
		return voiceChannels{}, false
	}

	if problems := au.Validate(config); len(problems) > 0 {
		for _, problem := range problems {
			report.Add(doctor.Check{
				Name: language.Say(text.CheckConfiguration), Level: doctor.Fail,
				Detail: problem.Describe(language),
				Fix:    language.Say(text.DoctorConfigInvalidFix),
			})
		}
		return voiceChannels{}, false
	}

	return voiceChannels{
		main:       config.MainVoiceChannelID,
		ghost:      config.GhostVoiceChannelID,
		control:    config.ControlTextChannelID,
		enabled:    config.Enabled,
		moveGhosts: config.AutoMoveGhosts,
	}, true
}

// voiceChannels is what the channel and permission checks need.
type voiceChannels struct {
	main    string
	ghost   string
	control string
	enabled bool
	// moveGhosts is whether the ghost channel is used at all (#161).
	moveGhosts bool
}

// checkChannels reports on the three channels a guild configures.
func (d *Doctor) checkChannels(report *doctor.Report, channels voiceChannels, configured bool, language text.Language) {
	if !configured {
		return
	}

	if !channels.enabled {
		report.Add(doctor.Check{
			Name: language.Say(text.CheckEnabled), Level: doctor.Warn,
			Detail: language.Say(text.DoctorDisabled),
			Fix:    language.Say(text.DoctorDisabledFix),
		})
	}

	for _, entry := range []struct {
		name      text.Key
		channelID string
		required  bool
		fix       text.Key
	}{
		{text.CheckMainChannel, channels.main, true, text.DoctorSetChannelFix},
		{text.CheckGhostChannel, channels.ghost, true, text.DoctorSetChannelFix},
		{text.CheckControlChannel, channels.control, false, text.DoctorControlChannelFix},
	} {
		name := language.Say(entry.name)
		switch {
		case entry.name == text.CheckGhostChannel && !channels.moveGhosts:
			// The dead stay muted in the main channel, so no ghost channel is
			// needed, and one that is still set is not used (#161).
			report.Add(doctor.Check{Name: name, Level: doctor.OK, Detail: language.Say(text.DoctorGhostNotUsed)})
		case entry.channelID == "" && entry.required:
			report.Add(doctor.Check{
				Name: name, Level: doctor.Fail,
				Detail: language.Say(text.DoctorNotSet), Fix: language.Say(entry.fix),
			})
		case entry.channelID == "":
			report.Add(doctor.Check{
				Name: name, Level: doctor.Warn,
				Detail: language.Say(text.DoctorNotSet), Fix: language.Say(entry.fix),
			})
		default:
			report.Add(d.describeChannel(name, entry.channelID, language))
		}
	}
}

// describeChannel reports whether a configured channel still exists.
//
// A channel that was deleted after being configured is the failure nobody
// thinks to look for, because the configuration still names it.
func (d *Doctor) describeChannel(name, channelID string, language text.Language) doctor.Check {
	if d.bot.PrimarySession == nil {
		return doctor.Check{Name: name, Level: doctor.Warn,
			Detail: language.Say(text.DoctorChannelUnconfirmed)}
	}

	channel, err := d.bot.PrimarySession.State.Channel(channelID)
	if err != nil || channel == nil {
		return doctor.Check{
			Name: name, Level: doctor.Fail,
			Detail: language.Say(text.DoctorChannelMissing, channelID),
			Fix:    language.Say(text.DoctorChannelMissingFix),
		}
	}

	return doctor.Check{Name: name, Level: doctor.OK, Detail: fmt.Sprintf("<#%s>", channelID)}
}

// checkPermissions reports the effective permissions on the voice channels.
func (d *Doctor) checkPermissions(report *doctor.Report, channels voiceChannels, configured bool, language text.Language) {
	if !configured || channels.main == "" || (channels.moveGhosts && channels.ghost == "") {
		return
	}
	if d.bot.PrimarySession == nil {
		return
	}

	// A ghost channel nobody is moved into needs no permissions (#161).
	ghost := ""
	if channels.moveGhosts {
		ghost = channels.ghost
	}
	reports := permission.Audit(d.bot.voiceChannelPermissions(channels.main, ghost)...)
	if len(reports) == 0 {
		report.Add(doctor.Check{
			Name: language.Say(text.CheckPermissions), Level: doctor.OK,
			Detail: language.Say(text.DoctorPermissionsGranted),
		})
		return
	}

	report.Add(doctor.Check{
		Name: language.Say(text.CheckPermissions), Level: doctor.Fail,
		Detail: permission.Summary(language, reports),
		Fix:    language.Say(text.DoctorPermissionsFix),
	})
}

// checkCrewmates reports whether players can choose their crewmate in the
// control channel.
func (d *Doctor) checkCrewmates(report *doctor.Report, channels voiceChannels, configured bool, language text.Language) {
	if !configured || channels.control == "" || d.bot.PrimarySession == nil {
		return
	}
	name := language.Say(text.CheckCrewmateMenu)

	effective, err := d.bot.PrimarySession.UserChannelPermissions(
		d.bot.PrimarySession.State.User.ID, channels.control)
	if err != nil {
		effective = 0
	}
	if missing := permission.Missing(effective, permission.ControlChannelNeeds); len(missing) > 0 {
		report.Add(doctor.Check{
			Name: name, Level: doctor.Fail,
			Detail: permission.Summary(language, []permission.Report{{
				Channel: permission.Channel{Purpose: text.PurposeTextChannel, ID: channels.control, Effective: effective},
				Missing: missing,
			}}),
			Fix: language.Say(text.DoctorCrewmatePermissionsFix),
		})
		return
	}

	if d.bot.Crewmates == nil {
		report.Add(doctor.Check{
			Name: name, Level: doctor.Warn,
			Detail: language.Say(text.DoctorCrewmateUnavailable),
			Fix:    language.Say(text.DoctorCrewmateUnavailableFix),
		})
		return
	}

	uploaded, total := d.bot.Crewmates.EmojiStatus()
	if uploaded < total {
		report.Add(doctor.Check{
			Name: name, Level: doctor.Warn,
			Detail: language.Say(text.DoctorCrewmatePictures, channels.control, uploaded, total),
			Fix:    language.Say(text.DoctorCrewmatePicturesFix),
		})
		return
	}

	report.Add(doctor.Check{
		Name: name, Level: doctor.OK,
		Detail: language.Say(text.DoctorCrewmateReady, channels.control),
	})
}

// checkCapture reports whether a capture is paired, connected and recent.
func (d *Doctor) checkCapture(report *doctor.Report, guildID string, language text.Language) {
	status, err := d.bot.AUVC.CaptureStatus(guildID)
	if err != nil {
		report.Add(doctor.Check{
			Name: language.Say(text.CheckCapture), Level: doctor.Fail,
			Detail: language.Say(text.DoctorCaptureUnreadable, err.Error()),
		})
		return
	}

	if !status.Paired {
		report.Add(doctor.Check{
			Name: language.Say(text.CheckCapture), Level: doctor.Warn,
			Detail: language.Say(text.DoctorCaptureNotPaired),
			Fix:    language.Say(text.DoctorCaptureNotPairedFix),
		})
		return
	}

	report.Add(doctor.Check{
		Name: language.Say(text.CheckCapture), Level: doctor.OK, Detail: language.Say(text.DoctorCapturePaired),
	})

	lastSeen, seen := d.bot.CaptureSessions.LastSeen(guildID)
	if !seen {
		report.Add(doctor.Check{
			Name: language.Say(text.CheckHeartbeat), Level: doctor.Warn,
			Detail: language.Say(text.DoctorHeartbeatNever),
			Fix:    language.Say(text.DoctorHeartbeatNeverFix),
		})
		return
	}

	since := time.Since(lastSeen)
	timeout := d.bot.captureTimeout(guildID)
	if since > timeout {
		report.Add(doctor.Check{
			Name: language.Say(text.CheckHeartbeat), Level: doctor.Fail,
			Detail: language.Say(text.DoctorHeartbeatStale, round(since), timeout),
			Fix:    language.Say(text.DoctorHeartbeatStaleFix),
		})
		return
	}

	report.Add(doctor.Check{
		Name: language.Say(text.CheckHeartbeat), Level: doctor.OK,
		Detail: language.Say(text.DoctorHeartbeatRecent, round(since), timeout),
	})

	report.Add(doctor.Check{
		Name: language.Say(text.CheckProtocol), Level: doctor.OK,
		Detail: language.Say(text.DoctorProtocol, protocol.Version),
	})
}

// checkSession reports what AUVC currently believes about the game.
func (d *Doctor) checkSession(report *doctor.Report, guildID string, language text.Language) {
	mode, phase, players := d.bot.CaptureSessions.Snapshot(guildID)
	name := language.Say(text.CheckGameState)

	if len(players) == 0 {
		report.Add(doctor.Check{
			Name: name, Level: doctor.Warn,
			Detail: language.Say(text.DoctorNoGameData),
			Fix:    language.Say(text.DoctorNoGameDataFix),
		})
		return
	}

	alive, dead := 0, 0
	for _, player := range players {
		if player.Disconnected {
			continue
		}
		if player.Alive {
			alive++
		} else {
			dead++
		}
	}

	detail := language.Say(text.DoctorGameState,
		describePhase(phase, language), alive, dead, describeMode(mode, language))

	level := doctor.OK
	fix := ""
	if mode != Running {
		level = doctor.Warn
		fix = language.Say(text.DoctorStartSessionFix)
	}
	report.Add(doctor.Check{Name: name, Level: level, Detail: detail, Fix: fix})
}

// checkBuild reports the version, which is the first thing to ask for in a bug
// report and the last thing anybody remembers to include.
func (d *Doctor) checkBuild(report *doctor.Report, language text.Language) {
	report.Add(doctor.Check{
		Name: language.Say(text.CheckBuild), Level: doctor.OK, Detail: d.bot.AUVC.Version(),
	})
}

// round trims a duration to something readable in a chat message.
func round(d time.Duration) time.Duration {
	if d < time.Minute {
		return d.Round(time.Second)
	}
	return d.Round(time.Minute)
}
