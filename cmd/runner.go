package main

import (
	"github.com/sirupsen/logrus"
	"hustoj/runner/src"
)

func main() {
	logrus.SetLevel(logrus.InfoLevel)
	task := runner.RunningTask{}
	task.Run()
}
