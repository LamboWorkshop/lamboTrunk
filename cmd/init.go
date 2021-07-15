package cmd

import (
	"io"
	"os"
	"time"

	binance "github.com/adshao/go-binance/v2"
	runtime "github.com/banzaicloud/logrus-runtime-formatter"
	api "github.com/segfault42/binance-api"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var (
	fLog   *os.File
	config SConfig     // struct config file
	client api.ApiInfo // client binance api

)

// SConfig config struct
type SConfig struct {
	LogFilePath string `json:"logFilePath"`
	// RefreshMarketMinute  int      `json:"minuteToAnalyze"`
	Assets               []string `json:"Assets"`
	CoinsToMonitorPath   string   `json:"coinsToMonitorPath"`
	RefreshMonitorMinute int64    `json:"refreshMonitorMinute"`
	IntervalRefresh      string   `json:"intervalRefresh"`
}

func loadConfig() (SConfig, error) {

	var config SConfig
	var err error

	if os.Getenv("APP_ENV") == "production" {
		viper.SetConfigFile("config_prod.json")
	} else {
		viper.SetConfigFile("config_dev.json")
	}

	viper.AddConfigPath(".")
	err = viper.ReadInConfig()
	if err != nil {
		return SConfig{}, err
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		return SConfig{}, err
	}

	return config, nil
}

func initConfig() SConfig {
	config, err := loadConfig()
	if err != nil {
		logrus.Fatalln("❌ loadConfig()", err)
	} else {
		logrus.Println("✅ loadConfig()")
	}
	return config
}

func initLog() {
	var err error

	// Create log file
	fLog, err = os.OpenFile(config.LogFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		logrus.Fatalf("❌ Error opening file: %v", err)
	}

	formatter := runtime.Formatter{ChildFormatter: &logrus.TextFormatter{
		FullTimestamp: true,
	}}
	formatter.Line = true
	logrus.SetFormatter(&formatter)
	// logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.DebugLevel)
	logrus.WithFields(logrus.Fields{
		"file": "main.go",
	})

	// Set output to file and stdout
	logrus.SetOutput(io.MultiWriter(os.Stdout, fLog))
	fLog.WriteString("======= New Log Session : " + time.Now().String() + " =======\n")

}

func init() {

	config = initConfig()
	initLog()

	DBClient = initDataBase()

	if os.Getenv("BINANCE_TESTNET") == "TRUE" {
		binance.UseTestnet = true
	}

	client = api.New()
	if client.Client.APIKey == "" {
		logrus.Fatalln("❌ binance api")
	} else {
		logrus.Println("✅ binance api")
	}
}
