package cmd

import (
	"github.com/sirupsen/logrus"
)

func logDebug(log string) {
	if devEnv == "DEV" {
		logrus.Debug(log)
	}
}
