package manage

import (
	"errors"
	"fmt"
	"github.com/SeanHeelan/Malamute/config"
	"github.com/SeanHeelan/Malamute/data"
	"github.com/SeanHeelan/Malamute/logging"
	"github.com/SeanHeelan/Malamute/monitor"
	"github.com/SeanHeelan/Malamute/mutate"
	"github.com/SeanHeelan/Malamute/resultproc"
	"github.com/SeanHeelan/Malamute/session"
	"log"
	"math/rand"
	"runtime"
	"time"
)

const (
	// Once 1/this tests have been processed from the last batch a new
	// batch will be requested
	REQ_THRESHOLD = 2
	// The number of seconds to wait once all tests have been processed
	// for any final messages from processing routines
	DRAIN_TIMEOUT = 2
)

func startMutator(s *session.Session, l *logging.Logs, errChan chan error,
	mutatorIn chan mutate.Request, mutatorOut chan data.TestCase) error {

	if s.Config.TestProcessing.Fuzzer == config.FUZZER_RADAMSA {
		radamsa := mutate.Radamsa{s, l}
		go radamsa.Run(mutatorIn, mutatorOut, errChan)
	} else if s.Config.TestProcessing.Fuzzer == config.FUZZER_RADAMSA_MULTIFILE {
		radamsaMultiFile := mutate.RadamsaMultiFile{s, l}
		go radamsaMultiFile.Run(mutatorIn, mutatorOut, errChan)
	} else if s.Config.TestProcessing.Fuzzer == config.FUZZER_NOP {
		nop := mutate.Nop{s.TestCasesDir}
		go nop.Run(mutatorIn, mutatorOut, errChan)
	} else {
		return errors.New(fmt.Sprintf("Invalid fuzzer selector %d",
			s.Config.TestProcessing.Fuzzer))
	}

	return nil
}

func isMultiFileMutator(mutator string) bool {
	if mutator == config.FUZZER_RADAMSA_MULTIFILE {
		return true
	}

	return false
}

func getMutationRequest(cfg config.TestProcessingConfig, seeds []string,
	batchSize int) mutate.Request {

	if !isMultiFileMutator(cfg.Fuzzer) {
		idx := rand.Int() % len(seeds)
		seedFile := seeds[idx]
		log.Printf("Selecting %s as the next seed file\n", seedFile)
		return mutate.Request{[]string{seedFile}, batchSize}
	}

	sources := []string{}
	gap := cfg.MultiFileFuzzerSeedCountMax - cfg.MultiFileFuzzerSeedCountMin
	seedsToUse := cfg.MultiFileFuzzerSeedCountMin + (rand.Int() % (gap + 1))
	for i := 0; i < seedsToUse; i++ {
		idx := rand.Int() % len(seeds)
		sources = append(sources, seeds[idx])
	}

	log.Printf("Selecting %v as the next seed files\n", sources)
	return mutate.Request{sources, batchSize}
}

func Run(s *session.Session, l *logging.Logs, seedFiles []string,
	termIndicator chan int) {

	rand.Seed(int64(s.Config.General.Seed))

	testCount := s.Config.TestProcessing.TestCount
	batchSize := s.Config.TestProcessing.BatchSize
	if testCount != 0 && batchSize > testCount {
		batchSize = testCount
		s.Config.TestProcessing.BatchSize = s.Config.TestProcessing.TestCount
	}

	errChan := make(chan error)
	mutatorIn := make(chan mutate.Request, 1)
	mutatorOut := make(chan data.TestCase, batchSize)

	if err := startMutator(s, l, errChan, mutatorIn, mutatorOut); err != nil {
		log.Printf("Error starting mutator %s", err)
		termIndicator <- 1
		return
	}

	// Figure out how many CPUs are available, and thus how many monitors
	// we may use
	numCpus := runtime.NumCPU()
	currMaxProcs := runtime.GOMAXPROCS(0)
	if currMaxProcs < numCpus {
		log.Printf("Increasing max processes from %d to %d\n", currMaxProcs,
			numCpus)
		runtime.GOMAXPROCS(numCpus)
	}

	// Start numCpus * 2 monitors
	monitorOut := make(chan data.TestCase, batchSize)
	log.Printf("Starting %d monitors as the CPU count is %d\n", numCpus*2,
		numCpus)
	for i := 0; i < numCpus*2; i++ {
		go monitor.ExitCode(s.Config, mutatorOut, monitorOut, errChan)
	}

	resultprocOut := make(chan data.TestCase, batchSize)
	go resultproc.LogFile(s.PreservationDir, monitorOut, resultprocOut,
		errChan)

	mutatorIn <- getMutationRequest(s.Config.TestProcessing,
		seedFiles, batchSize)

	fuzzFilesRequested := batchSize

	startTime := time.Now()

ManageLoop:
	for {
		var tc data.TestCase
		select {
		case tc = <-resultprocOut:
			if tc.BugFound {
				log.Printf("Potential bug: details %s\n", tc.PreservationDir)
				s.Stats.CrashCount++
			}
			s.Stats.TestCasesProcessed++

			for _, f := range tc.SeedFilePaths {
				s.Stats.AddTestCaseForSeed(f)
			}

			if tc.TestTimedOut {
				s.Stats.TimedOutTests++
			} else {
				s.Stats.AddExitCode(tc.ExitCode)
			}

			if err := s.Save(); err != nil {
				log.Printf("Failed to save session. Error: %s", err)
				break ManageLoop
			}
		case err := <-errChan:
			log.Printf("%s\n", err)
			break ManageLoop
		}

		if s.Stats.TestCasesProcessed%100 == 0 {
			log.Printf("%d fuzz files processed\n", s.Stats.TestCasesProcessed)
		}

		if s.Stats.TestCasesProcessed%batchSize == 0 {
			if err := s.LogSummary(); err != nil {
				log.Printf("Failed to log session summary. Error: %s", err)
				break ManageLoop
			}
		}

		if s.Stats.TestCasesProcessed%batchSize == 0 {
			duration := time.Now().Sub(startTime)
			log.Printf("Total time elapsed: %s\n", duration)
			batchAvg := int(duration.Seconds() /
				float64(s.Stats.TestCasesProcessed/batchSize))

			log.Printf("Average time per batch of %d: %d seconds\n",
				batchSize, batchAvg)
		}

		if testCount != 0 && s.Stats.TestCasesProcessed >= testCount {
			break
		}

		if fuzzFilesRequested-s.Stats.TestCasesProcessed <= batchSize/REQ_THRESHOLD {
			mutatorIn <- getMutationRequest(s.Config.TestProcessing,
				seedFiles, batchSize)
			fuzzFilesRequested += batchSize
		}
	}

	close(mutatorIn)

	log.Printf("%d fuzz files processed. Exiting...\n", s.Stats.TestCasesProcessed)

	termIndicator <- 1
}

func CoverAllSeedsOnce(s *session.Session, l *logging.Logs, seedFiles []string,
	termIndicator chan int) {

	rand.Seed(int64(s.Config.General.Seed))
	batchSize := s.Config.TestProcessing.BatchSize

	errChan := make(chan error)
	mutatorIn := make(chan mutate.Request, 1)
	mutatorOut := make(chan data.TestCase, batchSize)

	if err := startMutator(s, l, errChan, mutatorIn, mutatorOut); err != nil {
		log.Printf("Error starting mutator %s", err)
		termIndicator <- 1
		return
	}

	// Figure out how many CPUs are available, and thus how many monitors
	// we may use
	numCpus := runtime.NumCPU()
	currMaxProcs := runtime.GOMAXPROCS(0)
	if currMaxProcs < numCpus {
		log.Printf("Increasing max processes from %d to %d\n", currMaxProcs,
			numCpus)
		runtime.GOMAXPROCS(numCpus)
	}

	// Start numCpus * 2 monitors
	monitorOut := make(chan data.TestCase, batchSize)
	log.Printf("Starting %d monitors as the CPU count is %d\n", numCpus*2,
		numCpus)
	for i := 0; i < numCpus*2; i++ {
		go monitor.ExitCode(s.Config, mutatorOut, monitorOut, errChan)
	}

	resultprocOut := make(chan data.TestCase, batchSize)
	go resultproc.LogFile(s.PreservationDir, monitorOut, resultprocOut,
		errChan)

	idx := rand.Int() % len(seedFiles)
	seedFile := seedFiles[idx]
	// Remove the file from the s.Config.Seeds
	seedFiles[idx] = seedFiles[len(seedFiles)-1]
	seedFiles = seedFiles[0 : len(seedFiles)-1]

	log.Printf("Selecting %s as the next seed file\n", seedFile)

	mutatorIn <- mutate.Request{[]string{seedFile}, batchSize}

	fuzzFilesRequested := batchSize

	startTime := time.Now()
ManageLoop:
	for {
		var tc data.TestCase
		select {
		case tc = <-resultprocOut:
			if tc.BugFound {
				log.Printf("Potential bug: details %s\n", tc.PreservationDir)
				s.Stats.CrashCount++
			}
			s.Stats.TestCasesProcessed++

			for _, f := range tc.SeedFilePaths {
				s.Stats.AddTestCaseForSeed(f)
			}

			if tc.TestTimedOut {
				s.Stats.TimedOutTests++
			} else {
				s.Stats.AddExitCode(tc.ExitCode)
			}

			if err := s.Save(); err != nil {
				log.Printf("Failed to save session. Error: %s", err)
				break ManageLoop
			}
		case err := <-errChan:
			log.Printf("Error: %s\n", err)
			break ManageLoop
		}

		if s.Stats.TestCasesProcessed%100 == 0 {
			log.Printf("%d fuzz files processed\n", s.Stats.TestCasesProcessed)
		}

		if s.Stats.TestCasesProcessed%batchSize == 0 {
			if err := s.LogSummary(); err != nil {
				log.Printf("Failed to log session summary. Error: %s", err)
				break ManageLoop
			}
		}

		if s.Stats.TestCasesProcessed%batchSize == 0 {
			duration := time.Now().Sub(startTime)
			log.Printf("Total time elapsed: %s\n", duration)
			batchAvg := int(duration.Seconds() /
				float64(s.Stats.TestCasesProcessed/batchSize))

			log.Printf("Average time per batch of %d: %d seconds\n",
				batchSize, batchAvg)
			timeTilFinish := time.Duration(
				float64(batchAvg*len(seedFiles))) * time.Second
			log.Printf("Predicted time until finished: %s\n", timeTilFinish)
		}

		if fuzzFilesRequested-s.Stats.TestCasesProcessed <=
			batchSize/REQ_THRESHOLD {
			if len(seedFiles) == 0 {
				if fuzzFilesRequested == s.Stats.TestCasesProcessed {
					break
				}
			} else {
				idx = rand.Int() % len(seedFiles)
				seedFile = seedFiles[idx]

				// Remove the file from the s.Config.Seeds
				seedFiles[idx] = seedFiles[len(seedFiles)-1]
				seedFiles = seedFiles[0 : len(seedFiles)-1]

				log.Printf("Selecting %s as the next seed file\n", seedFile)
				mutatorIn <- mutate.Request{[]string{seedFile}, batchSize}

				fuzzFilesRequested += batchSize
			}
		}
	}

	close(mutatorIn)

	log.Printf("%d fuzz files processed. Exiting...\n", s.Stats.TestCasesProcessed)

	termIndicator <- 1
}
