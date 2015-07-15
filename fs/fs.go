package fs

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
// See: http://stackoverflow.com/questions/21060945/simple-way-to-copy-a-file-in-golang
func CopyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

// GetFilePaths returns a list of absolute paths, built by recursively
// traversing the directory specified by start_dir and looking for files with
// an extension in wanted_exts. Any errors encountered while walking the
// specified directory will be returned as the second return argument.
func GetFilePaths(startDir string, wantedExts []string) ([]string, error) {
	res := []string{}

	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)

		for idx := 0; idx < len(wantedExts); idx++ {
			if ext == wantedExts[idx] {
				res = append(res, path)
			}
		}
		return nil
	}

	err := filepath.Walk(startDir, walkFunc)

	return res, err
}

// ReadPathsFromFile takes in a file that should specify a valid path
// per line, and returns this list of paths as a slice. Each path is
// checked to ensure it actually points to a real file
func ReadPathsFromFile(file string) ([]string, error) {
	result := make([]string, 0)
	fd, err := os.Open(file)
	defer fd.Close()

	if err != nil {
		return result, err
	}

	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		path := scanner.Text()
		if fi, err := os.Stat(path); err != nil || !fi.Mode().IsRegular() {
			msg := fmt.Sprintf("%s is not a regular file", path)
			return result, errors.New(msg)
		}

		result = append(result, path)
	}

	return result, nil
}
