package cmd

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	binance "github.com/adshao/go-binance/v2"
	"github.com/k0kubun/pp"
	"github.com/sirupsen/logrus"
)

const (
	buy  = "BUY"
	sell = "SELL"
)

// func getAmountFromLastTransaction(latestTransaction transactionHistory) (float64, error) {
// 	amount, err := strconv.ParseFloat(latestTransaction.quantity, 64)
// 	if err != nil {
// 		logrus.Error(err)
// 		return 0.0, err
// 	}

// 	// fee, err := strconv.ParseFloat(latestTransaction.fee, 64)
// 	// if err != nil {
// 	// 	logrus.Error(err)
// 	// 	return 0.0, err
// 	// }

// 	// // pp.Println("amount:", amount, "amount-fee:", amount-fee)
// 	// amount -= fee

// 	return amount, nil
// }

func transaction(coin sCoinToMonitor) error {
	logrus.Info("Doing new transaction...")

	// Get the current candle
	depth, err := client.GetDepth(coin.pair, config.IntervalRefresh, 0, 1)
	if err != nil {
		logrus.Error(err)
		return err
	}

	if len(depth) == 0 {
		err = errors.New("GetDepth() returned an empty array")
		logrus.Error(err)
		return err
	}

	pp.Println(coin, depth[0])

	// Get openPrice in float
	open, err := strconv.ParseFloat(depth[0].Open, 64)
	if err != nil {
		logrus.Error(err)
		return err
	}

	// Get the latest transaction stored in DB
	latestTransaction, err := getLatestTransactionInDB(DBClient, coin.pair)
	if err != nil {
		return err
	}

	// GET CATH
	cath, err := getCATHFromDB(coin.pair + "_cath_history")
	if err != nil {
		return err
	}

	logrus.Info("\nlatest transaction : ", latestTransaction.side,
		"\nopen : ", open,
		"\ncath : ", cath.CustomAllTimeHigh)

	if open > cath.CustomAllTimeHigh && latestTransaction.side == buy {
		logrus.Debug("Set new CATH")

		if err := setNewCATH(coin.pair, latestTransaction); err != nil {
			return err
		}
	} else if open < cath.CustomAllTimeHigh && latestTransaction.side == buy {
		logrus.Debug("Sell, store in DB")

		// Get amount from the last transaction to invest the same amount
		// amount, err := getAmountFromLastTransaction(latestTransaction)
		// if err != nil {
		// 	return err
		// }

		amount, err := getAmountToTrade(coin.leftAsset, coin.percentInvest)
		if err != nil {
			return err
		}

		amountFloat, err := strconv.ParseFloat(amount, 64)
		if err != nil {
			return err
		}

		// substract fee (amountFromLastTransaction - fees)
		// fee, err := strconv.ParseFloat(latestTransaction.fee, 64)
		// if err != nil {
		// 	logrus.Error(err)
		// 	return err
		// }
		// amount -= fee

		resp, err := sellCoin(coin.pair, coin.leftAsset, amountFloat)
		if err != nil {
			return err
		}

		// Store transacation to DB
		if _, err := saveTransactionToDB(resp, coin.pair, true); err != nil {
			return err
		}

	} else if open > cath.CustomAllTimeHigh && latestTransaction.side == sell {
		logrus.Debug("Buy, store in DB, set new CATH")

		amount, err := getAmountToTrade(coin.rightAsset, coin.percentInvest)
		if err != nil {
			return err
		}
		// amount, err := getAmountFromLastTransaction(latestTransaction)
		// if err != nil {
		// 	return err
		// }

		// amount, err := client.GetFreeCoinAmount(coin.rightAsset)
		// if err != nil {
		// 	return err
		// }

		amountFloat, err := strconv.ParseFloat(amount, 64)
		if err != nil {
			return err
		}

		pp.Println("amountFloat:", amountFloat)
		pp.Println("leftAsset:", coin.rightAsset)

		resp, err := buyCoin(coin.pair, coin.rightAsset, amountFloat)
		if err != nil {
			return err
		}

		// Store transacation to DB
		trans, err := saveTransactionToDB(resp, coin.pair, true)
		if err != nil {
			return err
		}

		if err := setNewCATH(coin.pair, trans); err != nil {
			return err
		}
	} else {
		logrus.Info("Nothing to do")
	}

	return nil
}

func buyCoin(pair, rightAsset string, amountFloat float64) (*binance.CreateOrderResponse, error) {

	// Get price of coin
	resp, err := client.GetCurrentPrice(pair)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	price := resp.Price

	// Convert current price into float
	priceFloat, err := strconv.ParseFloat(price, 64)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

	stepSize, err := getStepSize(pair)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

	// Calulate amount to sell
	finalAmount := amountFloat / priceFloat
	finalAmount = finalAmount - (math.Mod(finalAmount, stepSize))

	pp.Println(client.GetBalance(rightAsset))
	pp.Println("final:", fmt.Sprintf("%.8f", finalAmount))

	respOrder, err := client.PlaceOrderMarketAmount(binance.SideTypeBuy, pair, fmt.Sprintf("%.8f", finalAmount), "real")
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

	// pp.Fatalln(respOrder)
	return respOrder, nil
}

func getStepSize(pair string) (float64, error) {
	lotSize, err := client.GetLotSize(pair)
	if err != nil {
		logrus.Error(err)
		return 0.0, err
	}

	stepSize, err := strconv.ParseFloat(lotSize.StepSize, 64)
	if err != nil {
		logrus.Error(err)
		return 0.0, err
	}

	return stepSize, nil
}

func sellCoin(pair, leftAsset string, amountFloat float64) (*binance.CreateOrderResponse, error) {

	stepSize, err := getStepSize(pair)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

	// Math to get the correct amount to sell
	amountFloat = amountFloat - (math.Mod(amountFloat, stepSize))

	amount := fmt.Sprintf("%.8f", amountFloat)

	pp.Println("Amount float", amountFloat)
	pp.Println("Sell coin : ", amount, "stepSize", stepSize)
	pp.Println(client.GetBalance(leftAsset))

	resp, err := client.PlaceOrderMarketAmount(binance.SideTypeSell, pair, amount, "real")
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

	return resp, nil
}

func doFirstTransaction(coin sCoinToMonitor) error {
	logrus.Info("No transactions for ", coin.pair, " Buying at the market price")

	amount, err := getAmountToTrade(coin.rightAsset, coin.percentInvest)
	if err != nil {
		return err
	}

	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		logrus.Error(err)
		return err
	}

	resp, err := buyCoin(coin.pair, coin.rightAsset, amountFloat)
	if err != nil {
		return err
	}

	if resp != nil {
		cath := setCATHFromLastTransaction(resp)

		if err := DBUpdateCATH(coin.pair+"_cath_history", cath); err != nil {
			return err
		}

		pp.Println(cath)

		if _, err := saveTransactionToDB(resp, coin.pair, true); err != nil {
			return err
		}
	} else {
		logrus.Info("Test buy transaction done")
	}

	return err
}

func saveTransactionToDB(resp *binance.CreateOrderResponse, pair string, botTransaction bool) (transactionHistory, error) {
	var transaction transactionHistory

	time.Sleep(time.Second * 10)
	pp.Println("resp: ", resp)
	last, err := client.GetTransactionByOrderID(resp.OrderID, pair)
	if err != nil {
		logrus.Error(err)
		return transactionHistory{}, err
	}

	price := setCATHFromLastTransaction(resp).CustomAllTimeHigh

	// Save to Database
	transaction.orderID = resp.OrderID
	transaction.pair = resp.Symbol
	transaction.price = fmt.Sprintf("%.8f", price)
	transaction.quantity = resp.OrigQuantity
	transaction.timestamp = resp.TransactTime
	transaction.total = last.CummulativeQuoteQuantity
	transaction.botTransaction = botTransaction
	transaction.side = string(resp.Side)
	transaction.fee = fmt.Sprintf("%.8f", calcFee(resp))

	table := pair + "_" + strings.ToLower(string(resp.Side)) + "_history"

	if err := addTransactionToDB(table, transaction); err != nil {
		logrus.Error(err)
		return transactionHistory{}, err
	}

	return transaction, nil
}

func calcPriceAverage(resp *binance.CreateOrderResponse) float64 {
	price := 0.0

	for _, elem := range resp.Fills {
		tmp, _ := strconv.ParseFloat(elem.Price, 64)
		price += tmp
	}
	return price / float64(len(resp.Fills))
}

func calcFee(resp *binance.CreateOrderResponse) float64 {
	fee := 0.0

	for _, elem := range resp.Fills {
		tmp, _ := strconv.ParseFloat(elem.Commission, 64)
		fee += tmp
	}
	return fee
}

func setCATHFromLastTransaction(resp *binance.CreateOrderResponse) customAllTimeHigh {

	price := calcPriceAverage(resp)

	return customAllTimeHigh{
		Timestamp:         resp.TransactTime,
		Pair:              resp.Symbol,
		CustomAllTimeHigh: price,
	}
}
