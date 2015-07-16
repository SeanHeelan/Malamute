package resultproc

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SeanHeelan/Malamute/data"
	"github.com/SeanHeelan/Malamute/monitor"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	SIGILL  = 128 + 4
	SIGABRT = 128 + 6
	SIGFPE  = 128 + 8
	SIGKILL = 128 + 9
	SIGSEGV = 128 + 11
	SIGTERM = 128 + 15

	BUG_DESC_NAME = "bugdesc.json"
	STDOUT_NAME   = "stdout.data"
	STDERR_NAME   = "stderr.data"
)

// BugDescriptor provides information on a test case that is considered
// to trigger a bug. It will be marshalled and stored in the preservation
// directory for that bug, alongside the trigger and other pertinant
// information. All file names are relative to the directory in which the
// marshalled BugDescriptor is found.
type BugDescriptor struct {
	// TriggerFileName specifies the name of the file that triggers
	// the bug.
	TriggerFileName string
	// SeedFileName specifies the name of the file containing the seed file
	// used to generate this test. This is a copy of the original that will
	// have been moved into the preservation directory.
	SeedFileNames []string
	// OriginalSeedPath provides the full path to the original seed file
	OriginalSeedPaths []string
	// ApplicationPath specifies the path to the application in which the bug
	// was found
	ApplicationPath string
	// ApplicationEnv specifies any extra environment variables that were
	// used during the execution of the test
	ApplicationEnv []string
	// RunExitCode is the exit code recorded after running the application
	// on the trigger file
	RunExitCode int
	// RunExeSeconds specifies how long the test ran for before the bug was
	// triggered
	RunExeSeconds int
	// RunStdoutData contains the path to a file holding the data recorded
	// from STDOUT during the execution of the application on the test case
	RunStdoutPath string
	// RunStderrData contains the path to a file holding the data recorded
	// from STDERR during the execution of the application on the test case
	RunStderrPath string
	// OverallTestCaseCount gives the total number of tests generated,
	// including this one, at the time that the bug was recorded
	OverallTestCaseCount int
	// SeedFileTestCaseCount gives the total number of tests generated from
	// the seed file of the test case, including this one, at the time that
	// the bug was recorded
	SeedFileTestCaseCounts map[string]int
}

func NewBugDescriptor(testCase data.TestCase) BugDescriptor {
	b := BugDescriptor{}

	b.SeedFileTestCaseCounts = testCase.SeedFuzzCounts
	b.OverallTestCaseCount = testCase.TotalFuzzCount
	b.RunExeSeconds = testCase.ExeSeconds
	b.RunExitCode = testCase.ExitCode
	b.ApplicationEnv = testCase.ApplicationEnv
	b.ApplicationPath = testCase.ApplicationPath
	b.OriginalSeedPaths = testCase.SeedFilePaths

	return b
}

// LogFile saves crashing tests cases to the preservation directory and
// simply deletes non-crashing test cases. Crashing tests are stored in
// their own sub-directory of the preservation directory, along with the
// seed test from which they were generated. In this sub-directory LogFile
// will also store any data written to stderr and stdout during the
// execution of a crashing test case.
func LogFile(preserveDir string, in chan data.TestCase,
	out chan data.TestCase, errOut chan error) {

	for {
		testCase := <-in
		if len(testCase.SeedFilePaths) == 0 {
			close(out)
			break
		}

		switch testCase.ExitCode {
		case SIGABRT, SIGFPE, SIGKILL, SIGSEGV, SIGTERM, SIGILL,
			monitor.ASAN_EXITCODE:
			testCase.BugFound = true
			bugDesc := NewBugDescriptor(testCase)

			// Each crash gets its own output directory
			now := time.Now()
			fuzzFileBase := filepath.Base(testCase.FuzzFilePath)
			crashDirName := fmt.Sprintf("%d_%s", now.Unix(), fuzzFileBase)
			crashDirPath := filepath.Join(preserveDir, crashDirName)
			err := os.Mkdir(crashDirPath, 0777)

			if err != nil {
				msg := fmt.Sprintf("Could not create output directory %s"+
					"for the crash file %s", crashDirPath,
					testCase.FuzzFilePath)
				errOut <- errors.New(msg)
				continue
			}

			testCase.PreservationDir = crashDirPath

			// Store the original files
			for _, seedFilePath := range testCase.SeedFilePaths {
				origPathWithSlashes := filepath.ToSlash(seedFilePath)
				origPathWithUScores := strings.Replace(origPathWithSlashes, "/", "_", -1)
				storagePathForOrig := filepath.Join(crashDirPath, origPathWithUScores)

				fileData, err := ioutil.ReadFile(seedFilePath)
				if err != nil {
					msg := fmt.Sprintf("Could not read the original file %s"+
						"for the crash file %s. Error %s", seedFilePath,
						testCase.FuzzFilePath, err)
					errOut <- errors.New(msg)
					continue
				}

				err = ioutil.WriteFile(storagePathForOrig, fileData, 0777)
				if err != nil {
					msg := fmt.Sprintf("Could not move the file %s to %s. Error %s",
						seedFilePath, storagePathForOrig, err)
					errOut <- errors.New(msg)
					continue
				}

				bugDesc.SeedFileNames = append(bugDesc.SeedFileNames, origPathWithUScores)
			}

			// Store the fuzz file
			fileBase := filepath.Base(testCase.FuzzFilePath)
			newPath := filepath.Join(crashDirPath, fileBase)
			err = os.Rename(testCase.FuzzFilePath, newPath)
			if err != nil {
				msg := fmt.Sprintf("Could not move the fuzz file %s to %s. Error %s",
					testCase.FuzzFilePath, newPath, err)
				errOut <- errors.New(msg)
				continue
			}

			bugDesc.TriggerFileName = fileBase

			// Store the stdout data
			stdoutPath := filepath.Join(crashDirPath, STDOUT_NAME)
			stdoutFd, err := os.Create(stdoutPath)
			if err != nil {
				errOut <- err
			}
			writer := bufio.NewWriter(stdoutFd)
			for _, line := range testCase.RunStdout {
				fmt.Fprintln(writer, line)
			}
			writer.Flush()
			stdoutFd.Close()
			bugDesc.RunStdoutPath = stdoutPath
			// Store the stderr data
			stderrPath := filepath.Join(crashDirPath, STDERR_NAME)
			stderrFd, err := os.Create(stderrPath)
			if err != nil {
				errOut <- err
			}
			writer = bufio.NewWriter(stderrFd)
			for _, line := range testCase.RunStderr {
				fmt.Fprintln(writer, line)
			}
			writer.Flush()
			stderrFd.Close()
			bugDesc.RunStderrPath = stderrPath

			// Store the bug descriptor
			bugDescPath := filepath.Join(crashDirPath, BUG_DESC_NAME)
			fd, err := os.Create(bugDescPath)
			if err != nil {
				msg := fmt.Sprintf("Failed to create file %s: %s",
					bugDescPath, err)
				errOut <- errors.New(msg)
				continue
			}

			if jsonData, err := json.Marshal(bugDesc); err != nil {
				msg := fmt.Sprintf("Error marshalling data: %s", err)
				errOut <- errors.New(msg)
				fd.Close()
				continue
			} else {
				if _, err := fd.Write(jsonData); err != nil {
					msg := fmt.Sprintf("Failed to write bug descriptor: %s",
						err)
					errOut <- errors.New(msg)
					fd.Close()
					continue
				}
			}

			fd.Close()

			out <- testCase
			continue
		}

		testCase.BugFound = false
		err := os.Remove(testCase.FuzzFilePath)
		if err != nil {
			log.Printf("Failed to remove %s\n. Removed by the test?",
				testCase.FuzzFilePath)
		}

		out <- testCase
	}
}
