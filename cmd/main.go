package cmd

import (
	"database/sql"
	"os"
	"time"

	"github.com/jasonlvhit/gocron"
	"github.com/sirupsen/logrus"
)

var devEnv string = os.Getenv("LAMBOTRUNK")

func getBotStatus() (bool, error) {

	botStatus := ""
	request := `SELECT bot_status FROM bot_status where id=$1`

	row := DBClient.QueryRow(request, 1)
	err := row.Scan(&botStatus)
	switch err {
	case sql.ErrNoRows:
		logrus.Warn("No rows were returned!")
		return false, nil
	case nil:
		if botStatus == "on" {
			return false, nil
		} else {
			return true, nil
		}
	default:
		return false, err
	}
}

func setBotStatus() error {
	request := `
	INSERT INTO bot_status (bot_status)
	VALUES ($1)
	RETURNING id`
	id := 0
	err := DBClient.QueryRow(request, "off").Scan(&id)
	if err != nil {
		logrus.Error(err)
		return err
	}

	return nil
}

func checkBotStatus() (bool, error) {
	// check in db
	// if bot_status table is empty set to off
	// else get bot status
	exist, err := isEmtyTable("bot_status")
	if err != nil {
		return true, err
	}
	if exist {
		return true, setBotStatus()
	} else {
		return getBotStatus()
	}
}

func myTask() {
	pause, err := checkBotStatus()
	if err != nil {
		os.Exit(-1)
	}

	if !pause {
		coinsToMonitor, _ := getCoinsToMonitor()

		if len(coinsToMonitor) == 0 {
			logrus.Info("No coins to monitor...")
		} else {
			monitorCoins(coinsToMonitor)
		}
	} else {
		logrus.Info("Bot is sleeping ...")
	}
}

func Main(args []string) {

	if err := createBotStatusTable(DBClient); err != nil {
		logrus.Error(err)
		return
	}
	// Do the task at startup
	myTask()

	minute := 0
	hour := 0
	if config.RefreshMonitorMinute < 60 {
		minute = int(config.RefreshMonitorMinute)
	} else if config.RefreshMonitorMinute >= 60 {
		hour = int(config.RefreshMonitorMinute) / 60
		minute = 1
	}

	// Setup time for cron job
	now := time.Now().Local()
	// t := time.Date(now.Year(), now.Month(), now.Day(), now.Hour()+1, 1, 0, 0, time.Local)
	// t := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()+1, 0, 0, time.Local)
	t := time.Date(now.Year(), now.Month(), now.Day(), now.Hour()+hour, now.Minute()+minute, 0, 0, time.Local)

	// If hourly, begin at the next rounded hour
	if hour > 0 {
		timestamp := t.Unix()
		timestamp -= timestamp % (3600)
		t = time.Unix(timestamp, 0)
	}

	// exec every X minutes
	gocron.Every(uint64(config.RefreshMonitorMinute)).Minutes().From(&t).Do(myTask)
	<-gocron.Start()
}
