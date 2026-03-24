package main

import (
	"github.com/sirupsen/logrus"
)

// log is the package-level logger used by all agentjail code.
var log = logrus.New()

func initLogger() {
	log.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp:       true,
		DisableLevelTruncation: true,
		PadLevelText:           true,
	})
	log.SetLevel(logrus.InfoLevel)
}

func enableVerboseLogging() {
	log.SetLevel(logrus.DebugLevel)
}
