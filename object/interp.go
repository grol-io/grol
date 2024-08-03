package object

import (
	"errors"
)

var commands map[string]Extension

func init() {
	Init()
}

func Init() {
	commands = make(map[string]Extension)
}

func CreateCommand(cmd Extension) error {
	if cmd.MaxArgs != -1 && cmd.MinArgs > cmd.MaxArgs {
		return errors.New(cmd.Name + ": min args > max args")
	}
	if len(cmd.ArgTypes) < cmd.MinArgs {
		return errors.New(cmd.Name + ": arg types < min args")
	}
	commands[cmd.Name] = cmd
	return nil
}

func Commands() map[string]Extension {
	return commands
}
