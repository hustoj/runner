package runner

import (
	"github.com/sirupsen/logrus"
	"os"
)

func InitLogger(logPath string, debug bool) *logrus.Logger {
	log = logrus.New()

	if len(logPath) > 0 {
		file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Panicf("open log file failed! %v", err)
		}
		log.Out = file
	} else {
		log.Out = os.Stdout
	}

	log.Formatter = &logrus.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	}

	if !debug {
		log.SetLevel(logrus.WarnLevel)
	} else {
		log.SetLevel(logrus.DebugLevel)
	}

	return log
}
