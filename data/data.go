package data

// TestCase instances are created for each file output by a mutator. It
// represents a single test and it is passed along the pipeline from a
// mutator, through to a monitor, through to a result processor, and finally
// out to the test manager. Each component in the pipeline will fill in
// different parts of an instance, as specified by the comments on each field
// below.
type TestCase struct {
	// SeedFilePath is the path to the original seed file. It will be filled
	// in by the mutator
	SeedFilePaths []string
	// FuzzFilePath is the path to the fuzz file to be used when the test is
	// executed. It will be filled in by the mutator.
	FuzzFilePath string
	// SeedFuzzCount gives the number of tests that have been generated
	// overall from the file specified by SeedFilePath. It will be filled in
	// by the mutator. It includes the current test.
	SeedFuzzCounts map[string]int
	// TotalFuzzCount gives the number of tests that have been generated
	// overall across all seeds. It will be filled in by the mutator. It
	// includes the current test.
	TotalFuzzCount int

	// ApplicationPath specifies the path to the application in which the bug
	// was found. It will be filled in by the execution monitor.
	ApplicationPath string
	// ApplicationEnv specifies any extra environment variables that were
	// used during the execution of the test. It will be filled in by the
	// execution monitor
	ApplicationEnv []string
	// TestTimedOut indicates whether the test case killed by the execution
	// monitor because it was taking too long. This will be filled in by the
	// execution monitor.
	TestTimedOut bool
	// ExeSeconds gives the number of seconds that the test took to execute.
	// This will be filled in by the execution monitor if TestTimedOut is
	// false
	ExeSeconds int
	// ExitCode gives the exit code from the test if TestTimedOut is false.
	// it will be filled in by the execution monitor.
	ExitCode int
	// RunStdout provides the data written to STDOUT during the application
	// under test. It will be filled in by the execution monitor if
	// TestTimedOut is false and ExitCode is not zero.
	RunStdout []string
	// RunStderr provides the data written to STDERR during the application
	// under test. It will be filled in by the execution monitor if
	// TestTimedOut is false and ExitCode is not zero.
	RunStderr []string

	// BugFound will be filled in by the results processor and indicates
	// whether it considered this test case to trigger a potential bug or not.
	BugFound bool
	// PreservationDir specifies the directory in which pertinant
	// information regarding the test will be stored if this test case is
	// considered to trigger a bug. It will be filled in by the results
	// processor.
	PreservationDir string
}

func NewTestCase() TestCase {
	tc := TestCase{}
	tc.SeedFuzzCounts = make(map[string]int)
	return tc
}
