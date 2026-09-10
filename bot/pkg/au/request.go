package au

import (
	"errors"
	"fmt"
	"math"

	"github.com/bwmarrin/discordgo"
)

// Values keeps Discord option values in their native kinds. Channel, role and
// user snowflakes are strings in Discord's API, while booleans and integers
// remain typed values all the way into the application service.
type Values struct {
	Strings  map[string]string
	Booleans map[string]bool
	Integers map[string]int64
}

func (v Values) String(name string) (string, bool) {
	value, ok := v.Strings[name]
	return value, ok
}

func (v Values) Bool(name string) (bool, bool) {
	value, ok := v.Booleans[name]
	return value, ok
}

func (v Values) Integer(name string) (int64, bool) {
	value, ok := v.Integers[name]
	return value, ok
}

// Request is the transport-independent input to the /au application service.
type Request struct {
	GuildID string
	Group   string
	Command string
	Invoker Invoker
	Values  Values
}

// ParsePathAndValues validates Discord's command nesting and extracts the
// native option values. Discord normally guarantees this shape; validating it
// here keeps malformed fixtures or future registration drift from panicking.
func ParsePathAndValues(options []*discordgo.ApplicationCommandInteractionDataOption) (string, string, Values, error) {
	values := Values{
		Strings:  make(map[string]string),
		Booleans: make(map[string]bool),
		Integers: make(map[string]int64),
	}
	if len(options) != 1 || options[0] == nil {
		return "", "", values, errors.New("expected exactly one /au subcommand or group")
	}

	top := options[0]
	group := ""
	command := top.Name
	leaves := top.Options

	if top.Type == discordgo.ApplicationCommandOptionSubCommandGroup {
		group = top.Name
		if len(top.Options) != 1 || top.Options[0] == nil ||
			top.Options[0].Type != discordgo.ApplicationCommandOptionSubCommand {
			return "", "", values, fmt.Errorf("group %q must contain exactly one subcommand", group)
		}
		command = top.Options[0].Name
		leaves = top.Options[0].Options
	} else if top.Type != discordgo.ApplicationCommandOptionSubCommand {
		return "", "", values, fmt.Errorf("top-level option %q is not a subcommand", top.Name)
	}

	for _, option := range leaves {
		if option == nil {
			return "", "", values, errors.New("command contains an empty option")
		}
		if _, exists := values.Strings[option.Name]; exists {
			return "", "", values, fmt.Errorf("option %q occurs more than once", option.Name)
		}
		if _, exists := values.Booleans[option.Name]; exists {
			return "", "", values, fmt.Errorf("option %q occurs more than once", option.Name)
		}
		if _, exists := values.Integers[option.Name]; exists {
			return "", "", values, fmt.Errorf("option %q occurs more than once", option.Name)
		}

		switch option.Type {
		case discordgo.ApplicationCommandOptionString,
			discordgo.ApplicationCommandOptionChannel,
			discordgo.ApplicationCommandOptionRole,
			discordgo.ApplicationCommandOptionUser:
			value, ok := option.Value.(string)
			if !ok {
				return "", "", values, fmt.Errorf("option %q has an invalid string value", option.Name)
			}
			values.Strings[option.Name] = value

		case discordgo.ApplicationCommandOptionBoolean:
			value, ok := option.Value.(bool)
			if !ok {
				return "", "", values, fmt.Errorf("option %q has an invalid boolean value", option.Name)
			}
			values.Booleans[option.Name] = value

		case discordgo.ApplicationCommandOptionInteger:
			value, ok := option.Value.(float64)
			const (
				minInt64Float          = -1 << 63
				maxInt64ExclusiveFloat = 1 << 63
			)
			if !ok || math.Trunc(value) != value || value < minInt64Float || value >= maxInt64ExclusiveFloat {
				return "", "", values, fmt.Errorf("option %q has an invalid integer value", option.Name)
			}
			values.Integers[option.Name] = int64(value)

		default:
			return "", "", values, fmt.Errorf("option %q has unsupported Discord type %d", option.Name, option.Type)
		}
	}

	return group, command, values, nil
}
