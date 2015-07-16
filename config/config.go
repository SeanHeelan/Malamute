package config

import (
	"code.google.com/p/gcfg"
	"errors"
	"fmt"
	"github.com/SeanHeelan/Malamute/arggen"
	"os"
	"strings"
)

const (
	FUZZER_RADAMSA_MULTIFILE = "radamsa_multifile"
	FUZZER_RADAMSA           = "radamsa"
	FUZZER_NOP               = "nop"

	RUNMODE_COVER_ALL_ONCE  = "cover_all_once"
	RUNMODE_INFINITE_RANDOM = "infinite_random"

	INTERPRETER_ARGS_FUZZ_FILE_MARKER     = "XXX_FUZZFILE_XXX"
	INTERPRETER_ARGS_FUZZ_FILE_DIR_MARKER = "XXX_FUZZFILEDIR_XXX"
)

type TestProcessingConfig struct {
	// Fuzzer specifies the fuzzer to use to generate tests from the seed
	// tests. See the FUZZER_* constants for valid values.
	Fuzzer string
	// MultiFileFuzzerSeedCountMin specifies the minimum number of seeds to
	// be feed to the mutator on each iteration of a multi-file mutator
	MultiFileFuzzerSeedCountMin int
	// MultiFileFuzzerSeedCountMax specifies the minimum number of seeds to
	// be feed to the mutator on each iteration of a multi-file mutator
	MultiFileFuzzerSeedCountMax int
	// BatchSize is used to configure how many test cases are generated
	// at a time, from each seed
	BatchSize int
	// TestCount specifies an upper limit on the total number of test
	// cases to process
	TestCount int
	// Mode specifies the run mode for seed test processing. It can either
	// be configured to cover each seed test once (generating BatchSize)
	// tests for each seed, or to run forever, randomly selecting seed
	// tests as it goes.
	Mode string
	// GenerateTestsInPlace indicates if the test cases should be
	// generated in the same directory as the seed tests, or in a
	// a clean directory.
	GenerateTestsInPlace bool
}

type Config struct {
	General struct {
		// Seed specifies the seed that will be used for any random number
		// generation required internally in the tool, or as an argument to
		// external tools e.g. radamsa
		Seed int
		// EnableDebugLog indicates whether verbose logging for the purposes
		// of debugging should be enabled or not
		EnableDebugLog bool
	}

	SeedTests struct {
		// Dir is a path to a directory that will traversed, including
		// sub-directories, in order to find files that end in an extension
		// found in the ValidExts slice. Each matching file will be
		// considered a seed test.
		Dir string
		// ValidExts, in combination with Dir, is used to find seed tests
		ValidExts []string
		// ListFile can be used instead of Dir+ValidExts to specify the paths
		// of all seed tests to be included. The paths should simply be listed
		// with a newline separating each.
		ListFile string
	}

	TestProcessing TestProcessingConfig

	Radamsa struct {
		// Mutations is the mutations argument to be passed to radamsa. See
		// the output of the `radamsa -l` command for details
		Mutations string
	}

	Interpreter struct {
		// Path specifies the path to the interpreter that will be
		// used to process each test case
		Path string
		// Args is a string specifying the arguments to be provided
		// to the interpreter. The string provided by the constant
		// INTERPRETER_ARGS_FUZZ_FILE_MARKER should appear at least once. This
		// marker will be replaced by the path to the test case on each
		// invocation of the interpreter. The string provided by the constant
		// INTERPRETER_ARGS_FUZZ_FILE_DIR_MARKER may appear any number of
		// times, and will be replaced by the directory in which the test
		// resides.
		Args string
		// ArgGen specifies the name of an argument generator function. Such
		// functions are provided by malamute to dynamically generate the
		// required arguments for each test case. The following are the
		// available values
		//	- FfJsRefTest : Generate arguments for the Firefox Javascript shell
		//		ref tests
		//
		ArgGen string
		// TestCasePathPrefix is the root directory containing all tests cases.
		// If ArgGen is provided then this may also be provided, and will be
		// passed to the argument generator, for its use
		TestCaseRootDir string
		// Timeout indicates the maximum run time, in seconds, of a single
		// instantiation of the interpreter
		Timeout int
	}
}

func Load(path string) (*Config, error) {
	var cfg Config
	if err := gcfg.ReadFileInto(&cfg, path); err != nil {
		return nil, err
	}

	if err := errorCheck(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ErrorCheck processes the configuration and checks it for any semantic
// errors. i.e. invalid combinations of options. It returns nil if everything
// is fine, or an error otherwise.
func errorCheck(cfg *Config) error {
	// General
	if cfg.General.Seed == 0 {
		return errors.New("Please use a seed other than 0, so we know " +
			"you've purposefully set it")
	}

	// SeedTests
	if len(cfg.SeedTests.Dir) == 0 && len(cfg.SeedTests.ListFile) == 0 {
		return errors.New("Seed tests must be specified via a directory" +
			" or list file")
	}

	if len(cfg.SeedTests.Dir) != 0 && len(cfg.SeedTests.ListFile) != 0 {
		return errors.New("You cannot specify both a tests directory and " +
			"file")
	}

	if len(cfg.SeedTests.ValidExts) == 0 && len(cfg.SeedTests.Dir) != 0 {
		return errors.New("One or more valid test extensions must be " +
			"provided")
	}

	if len(cfg.SeedTests.ValidExts) != 0 && len(cfg.SeedTests.ListFile) != 0 {
		return errors.New("It is not useful to specify the valid " +
			"extensions along with a tests file. All tests in the file " +
			"will be used.")
	}

	// TestProcessing
	cfg.TestProcessing.Fuzzer = strings.ToLower(cfg.TestProcessing.Fuzzer)
	if cfg.TestProcessing.Fuzzer != FUZZER_RADAMSA &&
		cfg.TestProcessing.Fuzzer != FUZZER_RADAMSA_MULTIFILE &&
		cfg.TestProcessing.Fuzzer != FUZZER_NOP {
		return errors.New(fmt.Sprintf("Invalid fuzzer selector %s",
			cfg.TestProcessing.Fuzzer))
	}

	if cfg.TestProcessing.Fuzzer == FUZZER_RADAMSA_MULTIFILE &&
		(cfg.TestProcessing.MultiFileFuzzerSeedCountMin == 0 ||
			cfg.TestProcessing.MultiFileFuzzerSeedCountMax == 0) {
		return fmt.Errorf("The MultiFileFuzzerSeedCounts must be greater" +
			" than 0")
	}

	if cfg.TestProcessing.BatchSize == 0 {
		return errors.New("Set the batch size to something greater than 0")
	}

	cfg.TestProcessing.Mode = strings.ToLower(cfg.TestProcessing.Mode)
	if cfg.TestProcessing.Mode != RUNMODE_COVER_ALL_ONCE &&
		cfg.TestProcessing.Mode != RUNMODE_INFINITE_RANDOM {
		return errors.New(fmt.Sprintf("Invalid mode %s",
			cfg.TestProcessing.Mode))
	}

	// Interpreter
	if len(cfg.Interpreter.Path) == 0 {
		return errors.New("You must specify an interpreter path")
	}

	usingArgs := len(cfg.Interpreter.Args) != 0
	usingArgGen := len(cfg.Interpreter.ArgGen) != 0
	if !usingArgs && !usingArgGen {
		return errors.New("An interpreter arguments string XOR an " +
			"argument generator must be provided")
	}

	if usingArgs && usingArgGen {
		return errors.New("An interpreter arguments string XOR an " +
			"argument generator must be provided")
	}

	if usingArgs && !strings.Contains(cfg.Interpreter.Args,
		INTERPRETER_ARGS_FUZZ_FILE_MARKER) {
		return errors.New(fmt.Sprintf("The provided interpreter "+
			"arguments string (%s) does not contain the correct fuzz file "+
			"marker", cfg.Interpreter.Args))
	}

	if usingArgGen && !(cfg.Interpreter.ArgGen == arggen.FF_JSREFTEST ||
		cfg.Interpreter.ArgGen == arggen.FF_JSREFTEST_IONEAGER ||
		cfg.Interpreter.ArgGen == arggen.D8_JSREFTEST) {
		return errors.New(fmt.Sprintf("Invalid argument generator: %s",
			cfg.Interpreter.ArgGen))
	}

	if usingArgGen && len(cfg.Interpreter.TestCaseRootDir) == 0 {
		return errors.New("You must specify the test case root directory")
	}

	if usingArgGen {
		if _, err := os.Stat(cfg.Interpreter.TestCaseRootDir); err != nil {
			return errors.New(fmt.Sprintf("Error reading %s, %s",
				cfg.Interpreter.TestCaseRootDir, err))
		}
	}

	if cfg.Interpreter.Timeout == 0 {
		return errors.New("You must specify the interpreter timeout")
	}

	return nil
}
