package logging

import (
	"github.com/SeanHeelan/malamute/session"
	"log"
	"os"
	"path"
)

const (
	DEBUG_LOG = "debug.log"
	DEBUG_PFX = "DEBUG "
)

type Logs struct {
	DebugLog *log.Logger
	DebugFd  *os.File
}

func Init(s *session.Session) (*Logs, error) {
	var debugLog *log.Logger
	var debugFd *os.File
	if s.Config.General.EnableDebugLog {
		logPath := path.Join(s.SessionDir, DEBUG_LOG)
		var err error
		debugFd, err = os.Create(logPath)
		if err != nil {
			return nil, err
		}

		debugLog = log.New(debugFd, DEBUG_PFX,
			log.Ldate|log.Ltime|log.Llongfile)
	}

	logs := Logs{debugLog, debugFd}
	return &logs, nil
}

func (l *Logs) Close() {
	if l.DebugFd != nil {
		l.DebugFd.Close()
	}
}

func (l *Logs) DEBUG(msg string) {
	if l.DebugLog == nil {
		return
	}

	l.DebugLog.Println(msg)
}

func (l *Logs) DEBUGF(fmt string, a ...interface{}) {
	if l.DebugLog == nil {
		return
	}

	l.DebugLog.Printf(fmt, a...)
}
