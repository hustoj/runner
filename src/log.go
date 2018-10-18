package runner

import (
	"github.com/sirupsen/logrus"
	"os"
)

var log *logrus.Logger

func init() {
	log = logrus.New()

	log.SetLevel(logrus.DebugLevel)
	log.Out = os.Stdout
}

func checkPanic(err error) {
	if err != nil {
		log.Panic(err)
	}
}
