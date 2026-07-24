package keystore

// https://github.com/calabash/calabash-android/blob/6bb3d9ac9eadf353dc7573c28a957e88e6669f67/ruby-gem/lib/calabash-android/helpers.rb

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/bitrise-io/go-utils/v2/pathutil"
)

const jarsigner = "/usr/bin/jarsigner"

// Runner wraps a v2 command.Factory + Logger, providing the small
// command-execution helpers this step used to get from the v1 command package.
type Runner struct {
	Logger     log.Logger
	CmdFactory command.Factory
}

// NewRunner returns a Runner backed by the provided logger and factory.
func NewRunner(logger log.Logger, cmdFactory command.Factory) Runner {
	return Runner{Logger: logger, CmdFactory: cmdFactory}
}

// Execute runs cmdSlice and streams the combined output to the logger.
func (r Runner) Execute(cmdSlice []string) error {
	if len(cmdSlice) == 0 {
		return fmt.Errorf("empty command")
	}

	cmd := r.CmdFactory.Create(cmdSlice[0], cmdSlice[1:], nil)
	r.Logger.Printf("=> %s\n", cmd.PrintableCommandArgs())

	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	r.Logger.Printf(out)

	return err
}

// ExecuteForOutput runs cmdSlice and returns the combined stdout+stderr as a string.
func (r Runner) ExecuteForOutput(cmdSlice []string) (string, error) {
	if len(cmdSlice) == 0 {
		return "", fmt.Errorf("empty command")
	}

	var outputBuf bytes.Buffer
	writer := io.MultiWriter(&outputBuf)
	cmd := r.CmdFactory.Create(cmdSlice[0], cmdSlice[1:], &command.Opts{
		Stdout: writer,
		Stderr: writer,
	})

	err := cmd.Run()
	if err != nil {
		err = fmt.Errorf("%s\n%s", outputBuf.String(), err)
	}

	return outputBuf.String(), err
}

// PrintableCommandArgs returns a shell-escaped representation of cmdSlice by
// constructing a throwaway command; kept as a helper so callers don't need
// their own copy.
func (r Runner) PrintableCommandArgs(cmdSlice []string) string {
	if len(cmdSlice) == 0 {
		return ""
	}

	cmd := r.CmdFactory.Create(cmdSlice[0], cmdSlice[1:], nil)

	return cmd.PrintableCommandArgs()
}

// Helper ...
type Helper struct {
	runner             Runner
	pathChecker        pathutil.PathChecker
	keystorePth        string
	keystorePassword   string
	alias              string
	signatureAlgorithm string
}

// NewHelper ...
func NewHelper(runner Runner, pathChecker pathutil.PathChecker, keystorePth, keystorePassword, alias string) (Helper, error) {
	if exist, err := pathChecker.IsPathExists(keystorePth); err != nil {
		return Helper{}, err
	} else if !exist {
		return Helper{}, fmt.Errorf("keystore not exist at: %s", keystorePth)
	}

	cmdSlice := []string{
		"keytool",
		"-list",
		"-v",

		"-keystore",
		keystorePth,
		"-storepass",
		keystorePassword,

		"-alias",
		alias,

		"-J-Dfile.encoding=utf-8",
		"-J-Duser.language=en-US",
	}

	out, err := runner.ExecuteForOutput(cmdSlice)
	if err != nil {
		return Helper{}, properError(err, out)
	}
	if out == "" {
		return Helper{}, fmt.Errorf("failed to read keystore, maybe alias (%s) or password (%s) is not correct", alias, "****")
	}

	signatureAlgorithm, err := findSignatureAlgorithm(runner.Logger, out)
	if err != nil {
		return Helper{}, err
	}
	if signatureAlgorithm == "" {
		return Helper{}, errors.New("failed to find signature algorithm")
	}

	return Helper{
		runner:             runner,
		pathChecker:        pathChecker,
		keystorePth:        keystorePth,
		keystorePassword:   keystorePassword,
		alias:              alias,
		signatureAlgorithm: signatureAlgorithm,
	}, nil
}

func (helper Helper) createSignCmd(buildArtifactPth, destBuildArtifactPth, privateKeyPassword string) ([]string, error) {
	split := strings.Split(helper.signatureAlgorithm, "with")
	if len(split) != 2 {
		return []string{}, fmt.Errorf("failed to parse signature algorithm: %s", helper.signatureAlgorithm)
	}
	split = strings.Split(split[1], "and")

	signingAlgorithm := "SHA256with" + split[0]
	digestAlgorithm := "SHA-256"

	cmdSlice := []string{
		jarsigner,
		"-sigfile",
		"CERT",

		"-sigalg",
		signingAlgorithm,
		"-digestalg",
		digestAlgorithm,

		"-keystore",
		helper.keystorePth,
		"-storepass",
		helper.keystorePassword,
	}

	if privateKeyPassword != "" {
		cmdSlice = append(cmdSlice, "-keypass", privateKeyPassword)
	}

	cmdSlice = append(cmdSlice, "-signedjar", destBuildArtifactPth, buildArtifactPth, helper.alias)

	return cmdSlice, nil
}

// SignBuildArtifact ...
func (helper Helper) SignBuildArtifact(buildArtifactPth, destBuildArtifactPth, privateKeyPassword string) error {
	if helper.pathChecker != nil {
		if exist, err := helper.pathChecker.IsPathExists(buildArtifactPth); err != nil {
			return err
		} else if !exist {
			return fmt.Errorf("Build Artifact not exist at: %s", buildArtifactPth)
		}
	}

	cmdSlice, err := helper.createSignCmd(buildArtifactPth, destBuildArtifactPth, privateKeyPassword)
	if err != nil {
		return err
	}

	helper.runner.Logger.Printf("=> %s", helper.runner.PrintableCommandArgs(secureSignCmd(cmdSlice)))

	out, err := helper.runner.ExecuteForOutput(cmdSlice)
	if err != nil {
		return properError(err, out)
	}
	if !strings.Contains(out, "jar signed.") {
		return errors.New(out)
	}

	return nil
}

// VerifyBuildArtifact ...
func (helper Helper) VerifyBuildArtifact(buildArtifactPth string) error {
	cmdSlice := []string{
		jarsigner,
		"-verify",
		"-verbose",
		"-certs",
		buildArtifactPth,
	}

	helper.runner.Logger.Printf("=> %s", helper.runner.PrintableCommandArgs(cmdSlice))

	out, err := helper.runner.ExecuteForOutput(cmdSlice)
	if err != nil {
		return properError(err, out)
	}
	if !strings.Contains(out, "jar verified.") {
		return errors.New(out)
	}

	return nil
}

func properError(err error, out string) error {
	var exitErr *command.ExitStatusError
	if errors.As(err, &exitErr) {
		return errors.New(out)
	}

	return err
}

func findSignatureAlgorithm(logger log.Logger, keystoreData string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(keystoreData))

	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "Signature algorithm name: ") {
			split := strings.Split(line, "Signature algorithm name: ")

			if len(split) < 2 {
				return "", fmt.Errorf("failed to expand signature algorithm from: %s", line)
			}

			alg := split[1]
			split = strings.Split(alg, " ")

			if len(split) > 1 {
				if logger != nil {
					logger.Warnf("🚨 Signature algorithm name contains unnecessary postfix: %s", alg)
					logger.Printf("Trimmed signature algorithm name: %s", split[0])
				}

				alg = split[0]
			}

			return alg, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", nil
}

func secureSignCmd(cmdSlice []string) []string {
	securedCmdSlice := []string{}
	secureNextParam := false
	for _, param := range cmdSlice {
		if secureNextParam {
			param = "***"
		}

		secureNextParam = (param == "-storepass" || param == "-keypass")
		securedCmdSlice = append(securedCmdSlice, param)
	}

	return securedCmdSlice
}
