package runner

import (
	"github.com/sirupsen/logrus"
	"os"
)

var log *logrus.Logger

func init() {
	log = logrus.New()

	file, err := os.OpenFile("/var/log/runner/runner.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		log.Panicln("open log file failed!", err)
	}
	log.Out = file
}

func Debug() {
	log.SetLevel(logrus.DebugLevel)
}

func checkPanic(err error) {
	if err != nil {
		log.Panic(err)
	}
}
