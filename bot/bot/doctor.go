package bot

import (
	"fmt"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/doctor"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/permission"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
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

// Diagnose runs every check for one guild.
func (d *Doctor) Diagnose(guildID string) (string, error) {
	var report doctor.Report

	d.checkDiscord(&report, guildID)
	config, configured := d.checkStorage(&report, guildID)
	d.checkChannels(&report, config, configured)
	d.checkPermissions(&report, config, configured)
	d.checkCapture(&report, guildID)
	d.checkSession(&report, guildID)
	d.checkBuild(&report)

	return report.Render(), nil
}

// checkDiscord reports whether the bot is connected and can see the guild.
func (d *Doctor) checkDiscord(report *doctor.Report, guildID string) {
	if d.bot.PrimarySession == nil {
		report.Add(doctor.Check{
			Name: "Discord", Level: doctor.Fail,
			Detail: "no Discord session",
			Fix:    "Restart AUVC and check `DISCORD_BOT_TOKEN`.",
		})
		return
	}

	guild, err := d.bot.PrimarySession.State.Guild(guildID)
	if err != nil || guild == nil {
		report.Add(doctor.Check{
			Name: "Discord", Level: doctor.Fail,
			Detail: "connected, but this server is not in the bot's cache",
			Fix:    "Remove and re-invite the bot, then try again.",
		})
		return
	}

	report.Add(doctor.Check{
		Name: "Discord", Level: doctor.OK,
		Detail: fmt.Sprintf("connected and watching **%s**", guild.Name),
	})
}

// checkStorage reports on SQLite and the guild's configuration, and hands back
// what the later checks need.
func (d *Doctor) checkStorage(report *doctor.Report, guildID string) (voiceChannels, bool) {
	version, err := d.bot.AUVC.SchemaVersion()
	if err != nil {
		report.Add(doctor.Check{
			Name: "Database", Level: doctor.Fail,
			Detail: "SQLite is not reachable: " + err.Error(),
			Fix:    "Check `AUVC_DATABASE_PATH` and that the volume is writable.",
		})
		return voiceChannels{}, false
	}

	report.Add(doctor.Check{
		Name: "Database", Level: doctor.OK,
		Detail: fmt.Sprintf("SQLite reachable, schema migration %d applied", version),
	})

	config, err := d.bot.AUVC.GuildConfig(guildID)
	if err != nil {
		report.Add(doctor.Check{
			Name: "Configuration", Level: doctor.Fail,
			Detail: "could not be read: " + err.Error(),
		})
		return voiceChannels{}, false
	}

	if problems := au.Validate(config); len(problems) > 0 {
		for _, problem := range problems {
			report.Add(doctor.Check{
				Name: "Configuration", Level: doctor.Fail,
				Detail: problem.Error(),
				Fix:    "Fix it with `/au settings`, or start over with `/au setup reset`.",
			})
		}
		return voiceChannels{}, false
	}

	return voiceChannels{
		main:    config.MainVoiceChannelID,
		ghost:   config.GhostVoiceChannelID,
		control: config.ControlTextChannelID,
		enabled: config.Enabled,
	}, true
}

// voiceChannels is what the channel and permission checks need.
type voiceChannels struct {
	main    string
	ghost   string
	control string
	enabled bool
}

// checkChannels reports on the three channels a guild configures.
func (d *Doctor) checkChannels(report *doctor.Report, channels voiceChannels, configured bool) {
	if !configured {
		return
	}

	if !channels.enabled {
		report.Add(doctor.Check{
			Name: "Enabled", Level: doctor.Warn,
			Detail: "AUVC is switched off for this server",
			Fix:    "Turn it on with `/au settings voice enabled:true`.",
		})
	}

	for _, entry := range []struct {
		name      string
		channelID string
		required  bool
		fix       string
	}{
		{"Main voice channel", channels.main, true, "Set it with `/au setup channels`."},
		{"Ghost voice channel", channels.ghost, true, "Set it with `/au setup channels`."},
		{"Control text channel", channels.control, false,
			"Set one with `/au setup channels` so AUVC can warn you when capture stops."},
	} {
		switch {
		case entry.channelID == "" && entry.required:
			report.Add(doctor.Check{
				Name: entry.name, Level: doctor.Fail, Detail: "not set", Fix: entry.fix,
			})
		case entry.channelID == "":
			report.Add(doctor.Check{
				Name: entry.name, Level: doctor.Warn, Detail: "not set", Fix: entry.fix,
			})
		default:
			report.Add(d.describeChannel(entry.name, entry.channelID))
		}
	}
}

// describeChannel reports whether a configured channel still exists.
//
// A channel that was deleted after being configured is the failure nobody
// thinks to look for, because the configuration still names it.
func (d *Doctor) describeChannel(name, channelID string) doctor.Check {
	if d.bot.PrimarySession == nil {
		return doctor.Check{Name: name, Level: doctor.Warn,
			Detail: "configured, but Discord is unavailable to confirm it"}
	}

	channel, err := d.bot.PrimarySession.State.Channel(channelID)
	if err != nil || channel == nil {
		return doctor.Check{
			Name: name, Level: doctor.Fail,
			Detail: fmt.Sprintf("<#%s> is configured but AUVC cannot see it", channelID),
			Fix:    "The channel may have been deleted. Set it again with `/au setup channels`.",
		}
	}

	return doctor.Check{Name: name, Level: doctor.OK, Detail: fmt.Sprintf("<#%s>", channelID)}
}

// checkPermissions reports the effective permissions on the voice channels.
func (d *Doctor) checkPermissions(report *doctor.Report, channels voiceChannels, configured bool) {
	if !configured || channels.main == "" || channels.ghost == "" {
		return
	}
	if d.bot.PrimarySession == nil {
		return
	}

	reports := permission.Audit(d.bot.voiceChannelPermissions(channels.main, channels.ghost)...)
	if len(reports) == 0 {
		report.Add(doctor.Check{
			Name: "Permissions", Level: doctor.OK,
			Detail: "all five voice permissions are granted on both channels",
		})
		return
	}

	report.Add(doctor.Check{
		Name: "Permissions", Level: doctor.Fail,
		Detail: permission.Summary(reports),
		Fix:    "Grant the missing permissions to the AUVC role, on the role or on the channel.",
	})
}

// checkCapture reports whether a capture is paired, connected and recent.
func (d *Doctor) checkCapture(report *doctor.Report, guildID string) {
	status, err := d.bot.AUVC.CaptureStatus(guildID)
	if err != nil {
		report.Add(doctor.Check{
			Name: "Capture", Level: doctor.Fail,
			Detail: "pairing state could not be read: " + err.Error(),
		})
		return
	}

	if !status.Paired {
		report.Add(doctor.Check{
			Name: "Capture", Level: doctor.Warn,
			Detail: "no capture app is paired",
			Fix:    "Run `/au capture pair` and type the code into the capture app.",
		})
		return
	}

	report.Add(doctor.Check{
		Name: "Capture", Level: doctor.OK, Detail: "paired",
	})

	lastSeen, seen := d.bot.CaptureSessions.LastSeen(guildID)
	if !seen {
		report.Add(doctor.Check{
			Name: "Heartbeat", Level: doctor.Warn,
			Detail: "capture is paired but has not connected since AUVC started",
			Fix:    "Start the capture app on the PC running Among Us.",
		})
		return
	}

	since := time.Since(lastSeen)
	timeout := d.bot.captureTimeout(guildID)
	if since > timeout {
		report.Add(doctor.Check{
			Name: "Heartbeat", Level: doctor.Fail,
			Detail: fmt.Sprintf("nothing heard for %s, past the %s timeout", round(since), timeout),
			Fix:    "Check that the capture app is still running and can reach this bot.",
		})
		return
	}

	report.Add(doctor.Check{
		Name: "Heartbeat", Level: doctor.OK,
		Detail: fmt.Sprintf("last message %s ago (timeout %s)", round(since), timeout),
	})

	report.Add(doctor.Check{
		Name: "Protocol", Level: doctor.OK,
		Detail: fmt.Sprintf("this bot speaks version %d, and capture is connected on it", protocol.Version),
	})
}

// checkSession reports what AUVC currently believes about the game.
func (d *Doctor) checkSession(report *doctor.Report, guildID string) {
	mode, phase, players := d.bot.CaptureSessions.Snapshot(guildID)

	if len(players) == 0 {
		report.Add(doctor.Check{
			Name: "Game state", Level: doctor.Warn,
			Detail: "no game data yet",
			Fix:    "Start Among Us with the capture app running.",
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

	detail := fmt.Sprintf("%s, %d alive and %d dead, session %s",
		describePhase(phase), alive, dead, mode)

	level := doctor.OK
	fix := ""
	if mode != Running {
		level = doctor.Warn
		fix = "Start managing voice with `/au session start`."
	}
	report.Add(doctor.Check{Name: "Game state", Level: level, Detail: detail, Fix: fix})
}

// checkBuild reports the version, which is the first thing to ask for in a bug
// report and the last thing anybody remembers to include.
func (d *Doctor) checkBuild(report *doctor.Report) {
	report.Add(doctor.Check{
		Name: "Build", Level: doctor.OK, Detail: d.bot.AUVC.Version(),
	})
}

// round trims a duration to something readable in a chat message.
func round(d time.Duration) time.Duration {
	if d < time.Minute {
		return d.Round(time.Second)
	}
	return d.Round(time.Minute)
}
