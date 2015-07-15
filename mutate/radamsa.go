package mutate

import (
	"errors"
	"fmt"
	"github.com/SeanHeelan/malamute/data"
	"github.com/SeanHeelan/malamute/logging"
	"github.com/SeanHeelan/malamute/session"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// Radamsa is a mutator based on the radamsa fuzzer, unsurprisingly. The
// WorkingDir variable specifies a directory into which fuzz files shall be
// written. The Seed variable specifies the seed value that will be passed
// directly to radamsa to seed its fuzzing engine.
type Radamsa struct {
	S *session.Session
	L *logging.Logs
}

// Run starts a work loop that consumes MutateRequests specifying a source file
// and the number of fuzz files to generate. As output it produces the paths to
// each of these fuzz files. On error a message will be sent on the errOut
// channel.
func (r *Radamsa) Run(in chan Request, out chan data.TestCase,
	errOut chan error) {

	cfg := r.S.Config
	// Used to change the seed on each iteration
	seedInc := 1
	testCasesGenerated := 0
	testCasesPerSeed := make(map[string]int)

	for {
		req := <-in

		if len(req.SourceFiles) == 0 {
			close(out)
			break
		}

		if len(req.SourceFiles) > 1 {
			log.Fatalf("For multiple source files use RadamsaMultiFile, not" +
				" Radamsa")
		}

		sourceFile := req.SourceFiles[0]
		fileName := filepath.Base(sourceFile)
		outputFileName := fmt.Sprintf("%%n_%s", fileName)

		var workingDir string
		if cfg.TestProcessing.GenerateTestsInPlace {
			workingDir = filepath.Dir(sourceFile)
		} else {
			workingDir = r.S.TestCasesDir
		}

		outputFilePath := filepath.Join(workingDir, outputFileName)

		countStr := strconv.Itoa(req.Count)

		seedStr := strconv.Itoa(cfg.General.Seed + seedInc)
		seedInc++
		cmdList := []string{}
		if len(cfg.Radamsa.Mutations) != 0 {
			cmdList = append(cmdList, "-m")
			cmdList = append(cmdList, cfg.Radamsa.Mutations)
		}
		cmdList = append(cmdList, "--seed")
		cmdList = append(cmdList, seedStr)
		cmdList = append(cmdList, "-n")
		cmdList = append(cmdList, countStr)
		cmdList = append(cmdList, "-o")
		cmdList = append(cmdList, outputFilePath)
		cmdList = append(cmdList, sourceFile)

		r.L.DEBUGF("Running radamsa with the following arguments : %s",
			cmdList)
		cmd := exec.Command("radamsa", cmdList...)

		if err := cmd.Run(); err != nil {
			msg := fmt.Sprintf("Error running radamsa: %s", err)
			err := errors.New(msg)
			errOut <- err
			continue
		}

		// If this is the first time we've seen this seed then initialize
		// its test case count to 0
		_, ok := testCasesPerSeed[sourceFile]
		if !ok {
			testCasesPerSeed[sourceFile] = 0
		}

		for i := 0; i < req.Count; i++ {
			expectedFileName := fmt.Sprintf("%d_%s", i+1, fileName)
			expectedFilePath := filepath.Join(workingDir, expectedFileName)

			if _, err := os.Stat(expectedFilePath); err != nil {
				err := errors.New(
					fmt.Sprintf("Fuzz file %s was not generated",
						expectedFilePath))
				errOut <- err
				continue
			}

			testCasesGenerated++
			testCasesPerSeed[sourceFile]++

			testCase := data.NewTestCase()
			testCase.FuzzFilePath = expectedFilePath
			testCase.SeedFilePaths = req.SourceFiles
			testCase.SeedFuzzCounts[sourceFile] = testCasesPerSeed[sourceFile]
			testCase.TotalFuzzCount = testCasesGenerated

			out <- testCase
		}
	}
}

// RadamsaMultiFile is a mutator based on the radamsa fuzzer. It generates
// test cases based on multiple input files, instead of a single input.
// WorkingDir variable specifies a directory into which fuzz files shall be
// written. The Seed variable specifies the seed value that will be passed
// directly to radamsa to seed its fuzzing engine.
type RadamsaMultiFile struct {
	S *session.Session
	L *logging.Logs
}

// Run starts a work loop that consumes MutateRequests specifying multiple
// source files and the number of fuzz files to generate. As output it
// produces the paths to each of these fuzz files. On error a message will
// be sent on the errOut channel.
func (r *RadamsaMultiFile) Run(in chan Request, out chan data.TestCase,
	errOut chan error) {

	cfg := r.S.Config
	// Used to change the seed on each iteration
	seedInc := 1
	testCasesGenerated := 0
	testCasesPerSeed := make(map[string]int)

	for {
		req := <-in

		if len(req.SourceFiles) == 0 {
			close(out)
			break
		}

		fileName := filepath.Base(req.SourceFiles[0])
		outputFileName := fmt.Sprintf("%%n_%s", fileName)

		var workingDir string
		if cfg.TestProcessing.GenerateTestsInPlace {
			workingDir = filepath.Dir(req.SourceFiles[0])
		} else {
			workingDir = r.S.TestCasesDir
		}

		outputFilePath := filepath.Join(workingDir, outputFileName)

		countStr := strconv.Itoa(req.Count)

		seedStr := strconv.Itoa(cfg.General.Seed + seedInc)
		seedInc++
		cmdList := []string{}
		if len(cfg.Radamsa.Mutations) != 0 {
			cmdList = append(cmdList, "-m")
			cmdList = append(cmdList, cfg.Radamsa.Mutations)
		}
		cmdList = append(cmdList, "--seed")
		cmdList = append(cmdList, seedStr)
		cmdList = append(cmdList, "-n")
		cmdList = append(cmdList, countStr)
		cmdList = append(cmdList, "-o")
		cmdList = append(cmdList, outputFilePath)
		for _, f := range req.SourceFiles {
			cmdList = append(cmdList, f)
		}

		r.L.DEBUGF("Running radamsa with the following arguments : %s",
			cmdList)
		cmd := exec.Command("radamsa", cmdList...)

		if err := cmd.Run(); err != nil {
			msg := fmt.Sprintf("Error running radamsa: %s", err)
			err := errors.New(msg)
			errOut <- err
			continue
		}

		for _, f := range req.SourceFiles {
			// If this is the first time we've seen this seed then initialize
			// its test case count to 0
			_, ok := testCasesPerSeed[f]
			if !ok {
				testCasesPerSeed[f] = 0
			}
		}

		for i := 0; i < req.Count; i++ {
			expectedFileName := fmt.Sprintf("%d_%s", i+1, fileName)
			expectedFilePath := filepath.Join(workingDir, expectedFileName)

			if _, err := os.Stat(expectedFilePath); err != nil {
				err := errors.New(
					fmt.Sprintf("Fuzz file %s was not generated",
						expectedFilePath))
				errOut <- err
				continue
			}

			testCase := data.NewTestCase()

			for _, f := range req.SourceFiles {
				testCasesPerSeed[f]++
			}

			for _, f := range req.SourceFiles {
				testCase.SeedFuzzCounts[f] = testCasesPerSeed[f]
			}

			testCasesGenerated++
			testCase.TotalFuzzCount++

			testCase.FuzzFilePath = expectedFilePath
			testCase.SeedFilePaths = req.SourceFiles

			out <- testCase
		}

	}
}
