package main

import (
	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-steplib/steps-sign-apk/keystore"
)

type zipalignConfiguration struct {
	zipalignPath string
	pageAlign    bool
}

func newZipalignConfiguration(zipalignPath string, pageAlign bool) *zipalignConfiguration {
	return &zipalignConfiguration{
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

	err := keystore.Execute(checkCmdSlice)
	if err != nil {
		if errorutil.IsExitStatusError(err) {
			return false, nil
		}
		return false, err
	}

	log.Printf("Artifact alignment confirmed.")
	return true, nil
}

func (config *zipalignConfiguration) zipalignArtifact(artifactPath, dstPath string) error {
	cmdSlice := []string{config.zipalignPath}
	if config.pageAlign {
		cmdSlice = append(cmdSlice, "-p")
	}
	cmdSlice = append(cmdSlice, "-f", "4", artifactPath, dstPath)
	log.Printf("=> %s", command.PrintableCommandArgs(false, cmdSlice))

	_, err := keystore.ExecuteForOutput(cmdSlice)
	return err
}
