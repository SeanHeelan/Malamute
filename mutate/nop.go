package mutate

import (
	"github.com/SeanHeelan/Malamute/data"
	"io/ioutil"
	"path/filepath"
)

type Nop struct {
	WorkingDir string
}

func (n *Nop) Run(in chan Request, out chan data.TestCase, errOut chan error) {
	testCasesGenerated := 0
	testCasesPerSeed := make(map[string]int)

	for {
		req := <-in

		if len(req.SourceFiles) == 0 {
			close(out)
			break
		}

		sourceFile := req.SourceFiles[0]
		fileName := filepath.Base(sourceFile)
		outputFilePath := filepath.Join(n.WorkingDir, fileName)

		fileData, err := ioutil.ReadFile(sourceFile)
		if err != nil {
			errOut <- err
			continue
		}
		ioutil.WriteFile(outputFilePath, fileData, 0777)

		// If this is the first time we've seen this seed then initialize
		// its test case count to 0
		_, ok := testCasesPerSeed[sourceFile]
		if !ok {
			testCasesPerSeed[sourceFile] = 0
		}

		testCasesGenerated++
		testCasesPerSeed[sourceFile]++

		testCase := data.TestCase{}
		testCase.FuzzFilePath = outputFilePath
		testCase.SeedFilePaths = []string{sourceFile}
		testCase.SeedFuzzCounts[sourceFile] = testCasesPerSeed[sourceFile]
		testCase.TotalFuzzCount = testCasesGenerated

		out <- testCase
	}
}
