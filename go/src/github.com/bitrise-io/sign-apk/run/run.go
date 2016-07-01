package run

import (
	"errors"
	"fmt"

	"github.com/bitrise-io/go-utils/cmdex"
	log "github.com/bitrise-io/sign-apk/logger"
)

// Execute ...
func Execute(cmdSlice []string) error {
	if len(cmdSlice) == 0 {
		return errors.New("no command specified")
	}

	prinatableCmd := cmdex.PrintableCommandArgs(false, cmdSlice)
	log.Details("=> %s", prinatableCmd)
	fmt.Println("")

	if len(cmdSlice) == 1 {
		out, err := cmdex.RunCommandAndReturnCombinedStdoutAndStderr(cmdSlice[0])
		log.Details(out)

		return err
	}

	out, err := cmdex.RunCommandAndReturnCombinedStdoutAndStderr(cmdSlice[0], cmdSlice[1:len(cmdSlice)]...)
	log.Details(out)

	return err
}

// ExecuteForOutput ...
func ExecuteForOutput(cmdSlice []string) (string, error) {
	if len(cmdSlice) == 0 {
		return "", errors.New("no command specified")
	}

	if len(cmdSlice) == 1 {
		return cmdex.RunCommandAndReturnCombinedStdoutAndStderr(cmdSlice[0])
	}

	return cmdex.RunCommandAndReturnCombinedStdoutAndStderr(cmdSlice[0], cmdSlice[1:len(cmdSlice)]...)
}
