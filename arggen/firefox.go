package arggen

import (
	"bytes"
	"fmt"
)

// FFJsRefTest_IonEager generates the required argumens to run one of
// Firefox's jsreftest tests. The process is simply to include (via the
// -f flag) the shell.js file in each directory/sub-directory between
// testBaseDir and the directory containing the test. It also appends
// the arguments --ion-eager and --fuzzing-safe.
func FFJsRefTest_IonEager(testBaseDir string, testPath string) (string, error) {

	args := []string{}
	var err error
	if args, err = get_shelljs_paths(testBaseDir, testPath); err != nil {
		return "", err
	}

	// Now we iterate over the list of paths in reverse order, and generate
	// the argument string
	var argBuf bytes.Buffer
	argBuf.WriteString("--fuzzing-safe --ion-eager ")

	for i := len(args) - 1; i >= 0; i-- {
		argBuf.WriteString(fmt.Sprintf("-f %s ", args[i]))
	}

	argBuf.WriteString(fmt.Sprintf("-f %s", testPath))

	return argBuf.String(), nil
}

// FFJsRefTest generates the required argumens to run one of
// Firefox's jsreftest tests. The process is simply to include (via the
// -f flag) the shell.js file in each directory/sub-directory between
// testBaseDir and the directory containing the test. It also appends
// the argument --fuzzing-safe.
func FFJsRefTest(testBaseDir string, testPath string) (string, error) {

	args := []string{}
	var err error
	if args, err = get_shelljs_paths(testBaseDir, testPath); err != nil {
		return "", err
	}

	// Now we iterate over the list of paths in reverse order, and generate
	// the argument string
	var argBuf bytes.Buffer
	argBuf.WriteString("--fuzzing-safe ")

	for i := len(args) - 1; i >= 0; i-- {
		argBuf.WriteString(fmt.Sprintf("-f %s ", args[i]))
	}

	argBuf.WriteString(fmt.Sprintf("-f %s", testPath))

	return argBuf.String(), nil
}
