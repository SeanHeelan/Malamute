package monitor

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/kballard/go-shellquote"
	"github.com/SeanHeelan/Malamute/arggen"
	"github.com/SeanHeelan/Malamute/config"
	"github.com/SeanHeelan/Malamute/data"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	ASAN_EXITCODE = 57
)

func scanToChannel(reader io.Reader, out chan []string) {
	data := []string{}
	scanner := bufio.NewScanner(bufio.NewReader(reader))

	// Take the first 200 lines of output
	for scanner.Scan() && len(data) <= 200 {
		data = append(data, scanner.Text())
	}

	out <- data
}

// ExitCode executes an interpreter on an input and records the exit code
// Bug: Data may be missed when reading from stdout and stderr. See
// https://codereview.appspot.com/6789043/
func ExitCode(cfg *config.Config, in chan data.TestCase,
	out chan data.TestCase, errOut chan error) {

	interpreterPath := cfg.Interpreter.Path
	interpreterArgs := cfg.Interpreter.Args
	var argGen arggen.GenFunc
	if len(interpreterArgs) == 0 {
		var err error
		argGen, err = arggen.GetGenerator(cfg.Interpreter.ArgGen)
		if err != nil {
			msg := fmt.Sprintf("Failed to get argument generator: %s",
				err)
			errOut <- errors.New(msg)
			return
		}
	}

	var asanEnvBuf bytes.Buffer
	asanEnvBuf.WriteString(fmt.Sprintf("exitcode=%d:", ASAN_EXITCODE))
	asanEnvBuf.WriteString("allocator_may_return_null=1")

	asanEnvMod := fmt.Sprintf("ASAN_OPTIONS=%s", asanEnvBuf.String())
	mallocCheckEnvMod := "MALLOC_CHECK_=2"
	environ := os.Environ()
	environ = append(environ, asanEnvMod)
	environ = append(environ, mallocCheckEnvMod)

	for {
		testCase := <-in
		if len(testCase.SeedFilePaths) == 0 {
			out <- testCase
			break
		}

		testCase.ApplicationEnv = append(testCase.ApplicationEnv, asanEnvMod)
		testCase.ApplicationEnv = append(testCase.ApplicationEnv,
			mallocCheckEnvMod)
		testCase.ApplicationPath = interpreterPath

		fuzzFile := testCase.FuzzFilePath
		base := filepath.Base(fuzzFile)
		dir := filepath.Dir(fuzzFile)
		now := time.Now().Unix()

		backupDirName := fmt.Sprintf("%d_%s", now, base)
		backupDirPath := filepath.Join(dir, backupDirName)
		os.Mkdir(backupDirPath, 0777)

		fileData, err := ioutil.ReadFile(fuzzFile)
		if err != nil {
			msg := fmt.Sprintf("Could not read the fuzz file %s. Error %s",
				fuzzFile, err)
			errOut <- errors.New(msg)
			continue
		}

		// Create a backup in case the file gets modified during the run
		backupPath := filepath.Join(backupDirPath, base)
		err = ioutil.WriteFile(backupPath, fileData, 0777)
		if err != nil {
			msg := fmt.Sprintf("Could not write %s to %s. Error %s",
				fuzzFile, backupPath, err)
			errOut <- errors.New(msg)
			continue
		}

		var argsStr string
		if argGen == nil {
			argsStr = strings.Replace(interpreterArgs,
				config.INTERPRETER_ARGS_FUZZ_FILE_MARKER, fuzzFile, -1)
			argsStr = strings.Replace(argsStr,
				config.INTERPRETER_ARGS_FUZZ_FILE_DIR_MARKER, dir, -1)
		} else {
			argsStr, err = argGen(cfg.Interpreter.TestCaseRootDir, fuzzFile)
			if err != nil {
				errOut <- err
				continue
			}
		}

		argsStrParts, err := shellquote.Split(argsStr)
		if err != nil {
			msg := fmt.Sprintf("Failed to parse target arguments : %s",
				argsStr)
			errOut <- errors.New(msg)
			continue
		}

		cmd := exec.Command(interpreterPath, argsStrParts...)
		cmd.Env = environ
		cmd.Dir = backupDirPath

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			msg := fmt.Sprintf("Error %s accessing stdout of command", err)
			errOut <- errors.New(msg)
			continue
		}
		stdoutChan := make(chan []string)
		go scanToChannel(stdout, stdoutChan)

		stderr, err := cmd.StderrPipe()
		if err != nil {
			msg := fmt.Sprintf("Error %s accessing stderr of command", err)
			errOut <- errors.New(msg)
			continue
		}
		stderrChan := make(chan []string)
		go scanToChannel(stderr, stderrChan)

		startTime := time.Now()
		if err := cmd.Start(); err != nil {
			msg := fmt.Sprintf("Error %s running %s on %s", err,
				interpreterPath, fuzzFile)
			err = errors.New(msg)
			errOut <- err
			continue
		}

		done := make(chan error)
		go func() {
			done <- cmd.Wait()
		}()

		var waitErr error
		select {
		case <-time.After(time.Duration(cfg.Interpreter.Timeout) *
			time.Second):
			// Process is taking too long, kill it
			if err := cmd.Process.Kill(); err != nil {
				log.Printf("Could not kill test process: %s", err)
			} else {
				log.Println("Process took too long to finish and was killed")
			}

			<-done
			<-stdoutChan
			<-stderrChan
			testCase.TestTimedOut = true
			out <- testCase

			if err := os.RemoveAll(backupDirPath); err != nil {
				errOut <- err
				continue
			}
			continue
		case waitErr = <-done:
		}

		testCase.TestTimedOut = false
		testCase.ExeSeconds = int(time.Now().Sub(startTime).Seconds())

		stdoutData := <-stdoutChan
		stderrData := <-stderrChan

		// In case the fuzz file was modified during the execution of the
		// test we write its original data back out. Should anything go wrong
		// before we get to do this, the backup still remains.
		err = ioutil.WriteFile(fuzzFile, fileData, 0777)
		if err != nil {
			msg := fmt.Sprintf("Could not write %s. Error %s", fuzzFile, err)
			errOut <- errors.New(msg)
			continue
		}

		if err := os.RemoveAll(backupDirPath); err != nil {
			log.Printf("Could not remove working directory %s : %s",
				backupDirPath, err)
		}

		if waitErr != nil {
			// Program returned exit code != 0
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					testCase.ExitCode = status.ExitStatus()
					testCase.RunStdout = stdoutData
					testCase.RunStderr = stderrData
					out <- testCase
					continue
				} else {
					// Failed to cast error code, we should never end up
					// in here
					msg := fmt.Sprintf("Could not translate error code "+
						"resulting from executing file %s", fuzzFile)
					err := errors.New(msg)
					errOut <- err
					continue
				}
			} else {
				// Failed to cast error code, we should never end up in here
				msg := fmt.Sprintf("Error %s executing %s on %s", waitErr,
					interpreterPath, fuzzFile)
				err := errors.New(msg)
				errOut <- err
				continue
			}
		} else {
			// Program returned exit code 0
			testCase.ExitCode = 0
			out <- testCase
			continue
		}
	}
}
