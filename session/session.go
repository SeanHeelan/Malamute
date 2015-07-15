package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/SeanHeelan/malamute/config"
	"github.com/SeanHeelan/malamute/fs"
	"io/ioutil"
	"os"
	"path"
	"strconv"
)

const (
	SESSION_FILE     = "session.json"
	SESSION_FILE_BCK = "session.json.bck"
	CONFIG_FILE      = "config.cfg"
	SUMMARY_FILE     = "summary.txt"
	TEST_CASES_DIR   = "test_cases"
	PRESERVATION_DIR = "crashes"
	DIR_PERMS        = 0755
)

type Stats struct {
	CrashCount                int
	TestCasesProcessed        int
	TimedOutTests             int
	ExitCodeCounts            map[string]int
	TestCasesProcessedPerSeed map[string]int
}

// AddTestCaseForSeed increments the test case counter for a particular seed
// file
func (s *Stats) AddTestCaseForSeed(seed string) {
	if _, ok := s.TestCasesProcessedPerSeed[seed]; ok {
		s.TestCasesProcessedPerSeed[seed]++
	} else {
		s.TestCasesProcessedPerSeed[seed] = 1
	}
}

func (s *Stats) AddExitCode(exitCode int) {
	exitCodeStr := strconv.Itoa(exitCode)
	if _, ok := s.ExitCodeCounts[exitCodeStr]; ok {
		s.ExitCodeCounts[exitCodeStr]++
	} else {
		s.ExitCodeCounts[exitCodeStr] = 1
	}
}

// Session contains enough information to restart a run of the fuzzer without
// reprocessing a set of already covered tests. This only makes sense when
// the run mode is to cover all tests once. Otherwise, the user can just
// restart the test with the same configuration and a different seed.
type Session struct {
	// SessionDir is the path from where this Session instance was loaded.
	// It is used when we want to re-save the instance to the same location,
	// without passing the path around.
	SessionDir      string
	TestCasesDir    string
	PreservationDir string
	Config          *config.Config
	Stats           Stats
}

// Save stores the session back to the same location it was loaded from
func (s *Session) Save() error {
	sessPath := path.Join(s.SessionDir, SESSION_FILE)

	if _, err := os.Stat(sessPath); err == nil {
		// If the session file already exists then back it up first
		backupPath := path.Join(s.SessionDir, SESSION_FILE_BCK)

		if err := fs.CopyFileContents(sessPath, backupPath); err != nil {
			return err
		}
	}

	fd, err := os.Create(sessPath)
	if err != nil {
		return err
	}
	defer fd.Close()

	if jsonData, err := json.Marshal(*s); err != nil {
		return err
	} else {
		if _, err := fd.Write(jsonData); err != nil {
			return err
		}
	}

	return nil
}

func (s *Session) LogSummary() error {
	logPath := path.Join(s.SessionDir, SUMMARY_FILE)

	fd, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer fd.Close()

	w := bufio.NewWriter(fd)
	defer w.Flush()

	fmt.Fprintf(w, "Total tests run: %d\n", s.Stats.TestCasesProcessed)
	fmt.Fprintf(w, "Crashes detected: %d\n", s.Stats.CrashCount)
	fmt.Fprintf(w, "Timed out tests: %d\n\n", s.Stats.TimedOutTests)

	fmt.Fprintf(w, "Exit code counts: \n")
	for exitCode, cnt := range s.Stats.ExitCodeCounts {
		fmt.Fprintf(w, "%s : %d\n", exitCode, cnt)
	}

	fmt.Fprint(w, "\nTests per seed:\n")
	for seed, cnt := range s.Stats.TestCasesProcessedPerSeed {
		fmt.Fprintf(w, "%s %d\n", seed, cnt)
	}

	return nil
}

func Create(sessDir string, configPath string) (*Session, error) {
	var err error
	var cfg *config.Config
	if cfg, err = config.Load(configPath); err != nil {
		return nil, err
	}

	if err = os.Mkdir(sessDir, DIR_PERMS); err != nil {
		return nil, err
	}

	test_cases_path := path.Join(sessDir, TEST_CASES_DIR)
	if err = os.Mkdir(test_cases_path, DIR_PERMS); err != nil {
		return nil, err
	}

	preservation_path := path.Join(sessDir, PRESERVATION_DIR)
	if err = os.Mkdir(preservation_path, DIR_PERMS); err != nil {
		return nil, err
	}

	testCounts := make(map[string]int)
	exitCodes := make(map[string]int)
	stats := Stats{0, 0, 0, exitCodes, testCounts}

	s := Session{sessDir, test_cases_path, preservation_path, cfg,
		stats}
	s.Save()

	newConfigPath := path.Join(sessDir, CONFIG_FILE)
	if err := fs.CopyFileContents(configPath, newConfigPath); err != nil {
		return nil, err
	}

	return &s, nil
}

// Resume unmarshals an existing Session, found in sessDir, and returns it
func Resume(sessDir string) (*Session, error) {
	sessPath := path.Join(sessDir, SESSION_FILE)

	data, err := ioutil.ReadFile(sessPath)
	if err != nil {
		return nil, err
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	s.SessionDir = sessDir

	return &s, nil
}
