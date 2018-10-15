package runner

import (
	"github.com/sirupsen/logrus"
	"os"
)

func init() {
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.DebugLevel)
}

func checkPanic(err error) {
	if err != nil {
		logrus.Panic(err)
	}
}
