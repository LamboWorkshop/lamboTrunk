package cmd

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	binance "github.com/adshao/go-binance/v2"
	"github.com/dariubs/percent"
	"github.com/sirupsen/logrus"
)

type sCoinToMonitor struct {
	leftAsset     string
	rightAsset    string
	pair          string
	percentInvest string
}

// func parseCoinToMonitor(monitor string) (sCoinToMonitor, string) {
// 	split := strings.Fields(monitor)
// 	var coinStruct sCoinToMonitor

// 	if len(split) == 2 {
// 		split = append(split, "0")
// 	}

// 	pairSplit := strings.Split(split[0], "/")
// 	if len(pairSplit) != 2 {
// 		return sCoinToMonitor{}, ""
// 	}

// 	coinStruct.leftAsset = pairSplit[0]
// 	coinStruct.rightAsset = pairSplit[1]
// 	coinStruct.pair = coinStruct.leftAsset + coinStruct.rightAsset

// 	if len(split) == 3 {
// 		if split[1] == "on" {
// 			coinStruct.percentInvest = split[2]
// 			return coinStruct, "on"
// 		} else if split[1] == "off" {
// 			coinStruct.percentInvest = split[2]
// 			return coinStruct, "off"
// 		} else {
// 			log.Println("Line is bad formatted :", monitor)
// 		}
// 	}

// 	return coinStruct, ""
// }

type sCoinsDB struct {
	id                    int
	pair, status, percent string
}

func getAllCoinsToMonitor() ([]sCoinsDB, error) {

	var coinsDB []sCoinsDB

	rows, err := DBClient.Query("SELECT * FROM coins_to_monitor;")
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tmp sCoinsDB

		err = rows.Scan(&tmp.id, &tmp.pair, &tmp.status, &tmp.percent)
		if err != nil {
			logrus.Error(err)
			return nil, err
		}

		coinsDB = append(coinsDB, tmp)

	}

	// get any error encountered during iteration
	err = rows.Err()
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

	return coinsDB, nil
}

func getCoinsToMonitor() ([]sCoinToMonitor, error) {
	var coinsToMonitor []sCoinToMonitor

	// Get info from DB instead

	coins, err := getAllCoinsToMonitor()
	if err != nil {
		return nil, err
	}

	for _, elem := range coins {
		if elem.status == "on" {

			var tmp sCoinToMonitor

			split := strings.Split(elem.pair, "/")

			tmp.pair = split[0] + split[1]
			tmp.percentInvest = elem.percent
			tmp.leftAsset = split[0]
			tmp.rightAsset = split[1]

			coinsToMonitor = append(coinsToMonitor, tmp)
		}
	}

	return coinsToMonitor, nil
}

func setNewCATH(pair string, trans transactionHistory) error {

	logDebug("SetNewCATH() : " + trans.price)

	open, err := strconv.ParseFloat(trans.price, 64)
	if err != nil {
		logrus.Error(err)
		return err
	}

	cath := customAllTimeHigh{
		Pair:              pair,
		Timestamp:         trans.timestamp,
		CustomAllTimeHigh: open,
	}

	return DBUpdateCATH(pair+"_cath_history", cath)
}

func getAmountToTrade(asset, percentToInvest string) (string, error) {
	// Get free amount of coin from binance
	freeCoin, err := client.GetFreeCoinAmount(asset)
	if err != nil {
		logrus.Error(err)
		return "", err
	}

	//Convert to float
	freeCoinFloat, err := strconv.ParseFloat(freeCoin, 64)
	if err != nil {
		logrus.Error(err)
		return "", err
	}

	// get percent to invest
	percentInvest, err := strconv.ParseFloat(percentToInvest, 64)
	if err != nil {
		logrus.Error(err)
		return "", err
	}

	quantity := fmt.Sprintf("%.8f", percent.PercentFloat(percentInvest, freeCoinFloat))

	return quantity, nil
}

func monitorCoin(coin sCoinToMonitor) error {

	logrus.Info("Monitoring :", coin.pair)

	// Check if transaction exist in DB.
	exist, err := checkIfTransactionExistInDB(DBClient, coin.pair)
	if err != nil {
		return err
	}

	// If manual transaction exist, store it in DB then pause the bot
	manualExist, err := checkForManualTransaction(coin)
	if err != nil {
		return err
	}
	if manualExist {
		logrus.Info("Manual Transaction was done. Bot is off for ", coin.pair)
		return nil
	}

	// If there is no transaction in DB (Sell/Buy). Do the first transaction
	if !exist {
		logDebug("Exec doFirstTransaction()")
		if err := doFirstTransaction(coin); err != nil {
			return err
		}
	} else { // else do new transaction
		err = transaction(coin)
		if err != nil {
			return err
		}
	}

	return nil
}

// DBSaveManualTransaction DBSaveManualTransaction
func dBSaveManualTransaction(DBClient *sql.DB, resp *binance.Order, pair string) (transactionHistory, error) {
	var transaction transactionHistory

	// Get fee from binance
	fee, err := client.GetFee()
	if err != nil {
		logrus.Error(err)
		return transactionHistory{}, err
	}

	// get fee in percent
	var finalfee float64 = float64(fee.MakerCommission) / 100

	// convert quantity in float64
	quantity, err := strconv.ParseFloat(resp.ExecutedQuantity, 64)
	if err != nil {
		logrus.Error(err)
		return transactionHistory{}, err
	}

	// Fill struct for DB
	transaction.botTransaction = false
	transaction.orderID = resp.OrderID
	transaction.pair = pair
	transaction.quantity = resp.ExecutedQuantity
	transaction.fee = fmt.Sprintf("%.8f", percent.PercentFloat(quantity, finalfee))
	transaction.side = string(resp.Side)
	transaction.timestamp = resp.Time
	transaction.total = resp.CummulativeQuoteQuantity
	transaction.price, err = client.GetPriceAtSpecificTime(pair, resp.Time)
	if err != nil {
		logrus.Error(err)
		return transactionHistory{}, err
	}

	// Store in Dtabase
	err = addTransactionToDB(pair+"_"+string(resp.Side)+"_history", transaction)
	if err != nil {
		return transactionHistory{}, err
	}

	return transaction, nil
}

func checkForManualTransaction(coin sCoinToMonitor) (bool, error) {
	resp, err := client.GetLastFilledTransaction(coin.pair)
	if err != nil {
		logrus.Error(err)
		return false, err
	}

	// check if order id is stored to DB
	// if transaction is empty, it means the latest binance transaction was done manually
	currentTransaction, err := dBGetTransactionByOrderID(DBClient, resp.OrderID, string(resp.Side))
	if currentTransaction != (transactionHistory{}) {
		// No manual transaction found. Or the latest manual transaction is already stored
		return false, err
	}

	// 1) Store to DB
	transaction, err := dBSaveManualTransaction(DBClient, resp, coin.pair)
	if err != nil {
		return false, err
	}

	// 2) Set CATH
	var cath customAllTimeHigh

	cath.Pair = coin.pair
	cath.Timestamp = transaction.timestamp
	cath.CustomAllTimeHigh, err = strconv.ParseFloat(transaction.price, 64)
	if err != nil {
		logrus.Error(err)
		return false, err
	}

	if err := DBUpdateCATH(coin.pair+"_cath_history", cath); err != nil {
		return false, err
	}

	// 3) Pause the bot for the current pair
	_, err = updateCoin(DBClient, coin.leftAsset+"/"+coin.rightAsset, "off")
	if err != nil {
		return false, err
	}

	return true, nil
}

func monitorCoins(coinsToMonitor []sCoinToMonitor) {

	for _, coinToMonitor := range coinsToMonitor {
		if err := createTables(coinToMonitor); err != nil {
			continue
		}
		monitorCoin(coinToMonitor)
	}
}
