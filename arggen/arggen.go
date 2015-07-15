package arggen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	FF_JSREFTEST_IONEAGER = "FfJsRefTest_IonEager"
	FF_JSREFTEST          = "FfJsRefTest"
	D8_JSREFTEST          = "D8JsRefTest"
)

type GenFunc func(string, string) (string, error)

func GetGenerator(genName string) (GenFunc, error) {
	switch genName {
	case FF_JSREFTEST:
		return FFJsRefTest, nil
	case FF_JSREFTEST_IONEAGER:
		return FFJsRefTest_IonEager, nil
	case D8_JSREFTEST:
		return D8JsRefTest, nil
	}

	msg := fmt.Sprintf("Unknown generator : %s", genName)
	return nil, errors.New(msg)
}

func get_shelljs_paths(testBaseDir string, testPath string) ([]string, error) {
	testBaseDir = filepath.Clean(testBaseDir)
	testPath = filepath.Clean(testPath)

	if !filepath.HasPrefix(testPath, testBaseDir) {
		msg := fmt.Sprintf("The test at %s does not have the provided "+
			"base directory %s as a prefix", testPath, testBaseDir)
		return nil, errors.New(msg)
	}
	args := []string{}

	// For every sub-directory between the base directory, and the directory
	// containing the test, we look for a 'shell.js' file, and if one is
	// found its path is recorded
	subDir := filepath.Dir(testPath)
	for subDir != testBaseDir {
		shellPath := filepath.Join(subDir, "shell.js")
		subDir = filepath.Clean(filepath.Join(subDir, "../"))

		if _, err := os.Stat(shellPath); err != nil {
			continue
		}

		args = append(args, shellPath)
	}

	shellPath := filepath.Join(subDir, "shell.js")
	if _, err := os.Stat(shellPath); err == nil {
		args = append(args, shellPath)
	} else {
		msg := fmt.Sprintf("%s should exist, but doesn't", shellPath)
		return nil, errors.New(msg)
	}

	return args, nil
}
