package main

import (
	"flag"
	"github.com/SeanHeelan/Malamute/config"
	"github.com/SeanHeelan/Malamute/fs"
	"github.com/SeanHeelan/Malamute/logging"
	"github.com/SeanHeelan/Malamute/manage"
	"github.com/SeanHeelan/Malamute/session"
	"log"
	"os"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "",
		"The config file to use")

	var sessionDirectory string
	flag.StringVar(&sessionDirectory, "dir", "",
		"The directory that will be used to store this session. If the run "+
			"mode is to cover all seed cases once then this directory may "+
			"already exist. If so, then the test will continue from where "+
			"it left off. If the run mode is any other then this directory "+
			"should not exist")

	flag.Parse()

	if len(sessionDirectory) == 0 {
		log.Fatal("You must specify a session directory")
	}

	var sess *session.Session
	var seedPaths []string
	if _, err := os.Stat(sessionDirectory); err == nil {
		// We are resuming an old session
		sess, err = session.Resume(sessionDirectory)
		if err != nil {
			log.Fatalf("Failed to load session from directory %s. Error: %s",
				sessionDirectory, err)
		}

		if sess.Config.TestProcessing.Mode != config.RUNMODE_COVER_ALL_ONCE {
			log.Fatalf("The session directory %s already exists, but the "+
				"run mode is not %s. Resuming sessions only makes sense "+
				"when covering all seed files once. Otherwise, just start a "+
				"new session with a different seed!", sessionDirectory,
				config.RUNMODE_COVER_ALL_ONCE)
		}

		seedPaths, err = loadSeeds(sess)
		if err != nil {
			log.Fatalf("Error loading seed tests %s", err)
		}
		seedPaths = filterSeeds(sess, seedPaths)
	} else {
		// We are starting a new session
		if len(configFile) == 0 {
			log.Fatal("You must specify a config file")
		}

		sess, err = session.Create(sessionDirectory, configFile)
		if err != nil {
			log.Fatalf("Failed to initialise session at %s. Error: %s",
				sessionDirectory, err)
		}

		seedPaths, err = loadSeeds(sess)
		if err != nil {
			log.Fatalf("Error loading seed tests %s", err)
		}
	}

	log.Printf("%d seed tests found\n", len(seedPaths))
	var logs *logging.Logs
	var err error
	if logs, err = logging.Init(sess); err != nil {
		log.Fatalf("Failed to initialise logging: %s", err)
	}
	defer logs.Close()

	termIndicator := make(chan int)
	if sess.Config.TestProcessing.Mode == config.RUNMODE_COVER_ALL_ONCE {
		log.Print("Covering all seeds once ...")
		go manage.CoverAllSeedsOnce(sess, logs, seedPaths, termIndicator)
	} else {
		if sess.Config.TestProcessing.TestCount == 0 {
			log.Println("Running tests until manual termination ...")
		} else {
			log.Printf("%d tests will be generated\n",
				sess.Config.TestProcessing.TestCount)
		}
		go manage.Run(sess, logs, seedPaths, termIndicator)
	}

	<-termIndicator
}

func loadSeeds(s *session.Session) (seedPaths []string, err error) {
	if len(s.Config.SeedTests.Dir) != 0 {
		dir := s.Config.SeedTests.Dir
		exts := s.Config.SeedTests.ValidExts
		log.Printf("Seed tests will be extracted from %s\n", dir)
		log.Printf("Searching for tests with the following extensions: %s\n",
			exts)
		seedPaths, err = fs.GetFilePaths(dir, exts)
	} else {
		file := s.Config.SeedTests.ListFile
		log.Printf("Seed tests will be extracted from %s\n", file)
		seedPaths, err = fs.ReadPathsFromFile(file)
	}

	return seedPaths, err
}

func filterSeeds(s *session.Session, seeds []string) (filtered []string) {
	for _, path := range seeds {
		if testCount, ok := s.Stats.TestCasesProcessedPerSeed[path]; !ok ||
			testCount < s.Config.TestProcessing.BatchSize {
			filtered = append(filtered, path)
		}
	}

	return filtered
}
