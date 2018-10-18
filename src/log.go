package runner

import (
	"github.com/sirupsen/logrus"
	"os"
)

var log *logrus.Logger

func init() {
	log = logrus.New()

	log.Out = os.Stdout
}

func Debug() {
	log.SetLevel(logrus.DebugLevel)
}

func checkPanic(err error) {
	if err != nil {
		log.Panic(err)
	}
}
