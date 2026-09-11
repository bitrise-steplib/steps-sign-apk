package main

import (
	"errors"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-steplib/steps-sign-apk/keystore"
)

type zipalignConfiguration struct {
	runner       keystore.Runner
	logger       log.Logger
	zipalignPath string
	pageAlign    bool
}

func newZipalignConfiguration(runner keystore.Runner, logger log.Logger, zipalignPath string, pageAlign bool) *zipalignConfiguration {
	return &zipalignConfiguration{
		runner:       runner,
		logger:       logger,
		zipalignPath: zipalignPath,
		pageAlign:    pageAlign,
	}
}

func (config *zipalignConfiguration) checkAlignment(artifactPath string) (bool, error) {
	checkCmdSlice := []string{config.zipalignPath}
	if config.pageAlign {
		checkCmdSlice = append(checkCmdSlice, "-p")
	}
	checkCmdSlice = append(checkCmdSlice, "-c", "4", artifactPath)

	err := config.runner.Execute(checkCmdSlice)
	if err != nil {
		var exitErr *command.ExitStatusError
		if errors.As(err, &exitErr) {
			return false, nil
		}

		return false, err
	}

	config.logger.Printf("Artifact alignment confirmed.")

	return true, nil
}

func (config *zipalignConfiguration) zipalignArtifact(artifactPath, dstPath string) error {
	cmdSlice := []string{config.zipalignPath}
	if config.pageAlign {
		cmdSlice = append(cmdSlice, "-p")
	}
	cmdSlice = append(cmdSlice, "-f", "4", artifactPath, dstPath)

	_, err := config.runner.ExecuteForOutput(cmdSlice)

	return err
}
