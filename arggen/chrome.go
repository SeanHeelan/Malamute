package arggen

import (
	"bytes"
	"fmt"
	"path"
)

// D8JsRefTest generates the required argumens to run one of
// Firefox's jsreftest tests in the D8 interpreter. The process is simply to
// include the shell.js file in each directory/sub-directory
// between testBaseDir and the directory containing the test. It also appends
// the arguments --expose_gc, --harmony, and the path
// testBaseDir/ffshellfuncs.js
func D8JsRefTest(testBaseDir string, testPath string) (string, error) {
	args := []string{}
	var err error
	if args, err = get_shelljs_paths(testBaseDir, testPath); err != nil {
		return "", err
	}

	// Now we iterate over the list of paths in reverse order, and generate
	// the argument string
	var argBuf bytes.Buffer
	argBuf.WriteString("--expose_gc ")
	argBuf.WriteString("--harmony ")
	argBuf.WriteString(path.Join(testBaseDir, "ffshellfuncs.js "))

	for i := len(args) - 1; i >= 0; i-- {
		argBuf.WriteString(fmt.Sprintf("%s ", args[i]))
	}

	argBuf.WriteString(fmt.Sprintf("%s", testPath))

	return argBuf.String(), nil
}
