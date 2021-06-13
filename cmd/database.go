package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/k0kubun/pp"
	// for psql
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

var (
	host     = os.Getenv("POSTGRES_HOST")
	port     = os.Getenv("POSTGRES_PORT")
	user     = os.Getenv("POSTGRES_USER")
	password = os.Getenv("POSTGRES_PASSWORD")
	dbname   = os.Getenv("POSTGRES_DB")
)

type transactionHistory struct {
	id             int64
	timestamp      int64
	quantity       string
	price          string
	pair           string
	orderID        int64
	total          string
	botTransaction bool
	side           string
	fee            string
}

type customAllTimeHigh struct {
	ID                int
	CustomAllTimeHigh float64
	Timestamp         int64
	Pair              string
}

// DBClient database client
var DBClient *sql.DB

// ConnectToDB func to init connection to DB
func ConnectToDB() (*sql.DB, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return db, err
	}

	err = db.Ping()
	if err != nil {
		return db, err
	}

	return db, err
}

func initDataBase() *sql.DB {
	db, err := ConnectToDB()
	if err != nil {
		logrus.Fatalln("❌ initDatabase():", err)
	} else {
		logrus.Println("✅ initDatabase()")
	}

	return db
}

// CreateTable create a table
func CreateTable(db *sql.DB, tableName, tableContent string) error {

	query := `CREATE TABLE IF NOT EXISTS ` + tableName + ` (` + tableContent + `);`

	stmt, err := db.Prepare(query)
	if err != nil {
		logrus.Error(err)
		return err
	}

	defer stmt.Close()

	_, err = stmt.Exec()
	if err != nil {
		logrus.Error(err)
		return err
	}

	return nil
}

func isEmtyTable(table string) (bool, error) {

	count := 0
	query := "SELECT COUNT(*) AS RowCnt FROM " + table

	row := DBClient.QueryRow(query)
	err := row.Scan(&count)
	if err != nil {
		logrus.Fatal(err)
	}

	if count == 0 {
		return true, nil
	}

	return false, nil
}

func addTransactionToDB(table string, transaction transactionHistory) error {
	pp.Println("add:", table, transaction)
	request := `
	INSERT INTO ` + table + ` (timestamp, price_dollar, quantity, total, pair, order_id, bot_transaction, side, fee)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	RETURNING id`
	id := 0
	err := DBClient.QueryRow(request, transaction.timestamp, transaction.price, transaction.quantity, transaction.total, transaction.pair, transaction.orderID, transaction.botTransaction, transaction.side, transaction.fee).Scan(&id)
	if err != nil {
		logrus.Error(err)
		return err
	}
	logDebug("addCATHToDB() New record ID is:" + strconv.Itoa(id))

	return nil
}

// DBUpdateCATH update CATH
func DBUpdateCATH(table string, CATH customAllTimeHigh) error {

	exist, err := isEmtyTable(table)
	if err != nil {
		return err
	}
	if exist {
		err := addCATHToDB(table, CATH)
		if err != nil {
			return err
		}
	} else {
		request := `UPDATE ` + table + ` SET custom_all_time_high = $1, timestamp = $2 WHERE id = $3 RETURNING id`
		id := 0
		err := DBClient.QueryRow(request, CATH.CustomAllTimeHigh, CATH.Timestamp, 1).Scan(&id)
		if err != nil {
			logrus.Error(err)
			return err
		}
		logDebug("DBUpdateCATH() New record ID is:" + strconv.Itoa(id))
	}

	return nil
}

func addCATHToDB(table string, CATH customAllTimeHigh) error {
	request := `
	INSERT INTO ` + table + ` (timestamp, custom_all_time_high, pair)
	VALUES ($1, $2, $3)
	RETURNING id`
	id := 0
	pp.Println(CATH)
	err := DBClient.QueryRow(request, CATH.Timestamp, CATH.CustomAllTimeHigh, CATH.Pair).Scan(&id)
	if err != nil {
		logrus.Error(err)
		return err
	}

	logDebug("addCATHToDB() New record ID is:" + strconv.Itoa(id))

	return nil
}

func createCATHTable(DBClient *sql.DB, tableName string) error {

	return CreateTable(DBClient, tableName, `id  SERIAL PRIMARY KEY,
		timestamp BIGINT,
		custom_all_time_high FLOAT(8),
		pair TEXT`)
}

func createTransactionTable(DBClient *sql.DB, tableName string) error {

	return CreateTable(DBClient, tableName, `id  SERIAL PRIMARY KEY,
		timestamp BIGINT,
		price_dollar TEXT,
		quantity TEXT,
		total TEXT,
		pair TEXT,
		order_id BIGINT,
		bot_transaction BOOL,
		side TEXT,
		fee TEXT`)
}

func getLatestTransactionInTable(DBClient *sql.DB, table string) (transactionHistory, error) {
	var transaction transactionHistory
	request := `select * from ` + table + ` order by timestamp desc limit 1`

	row := DBClient.QueryRow(request)
	err := row.Scan(&transaction.id, &transaction.timestamp, &transaction.price,
		&transaction.quantity, &transaction.total, &transaction.pair, &transaction.orderID, &transaction.botTransaction, &transaction.side, &transaction.fee)
	switch err {
	case sql.ErrNoRows:
		return transactionHistory{}, nil
	case nil:
		return transaction, nil
	default:
		return transactionHistory{}, err
	}
}

func getCATHFromDB(table string) (customAllTimeHigh, error) {

	var CATH customAllTimeHigh

	request := `select * from ` + table + ` order by custom_all_time_high desc limit 1`

	row := DBClient.QueryRow(request)
	err := row.Scan(&CATH.ID, &CATH.Timestamp, &CATH.CustomAllTimeHigh, &CATH.Pair)
	switch err {
	case sql.ErrNoRows:
		return customAllTimeHigh{}, nil
	case nil:
		return CATH, nil
	default:
		logrus.Error(err)
		return customAllTimeHigh{}, err
	}
}

func createTables(coinToMonitor sCoinToMonitor) error {
	if err := createTransactionTable(DBClient, coinToMonitor.pair+"_sell_history"); err != nil {
		logrus.Error(err)
		return err
	}
	if err := createTransactionTable(DBClient, coinToMonitor.pair+"_buy_history"); err != nil {
		logrus.Error(err)
		return err
	}
	if err := createCATHTable(DBClient, coinToMonitor.pair+"_cath_history"); err != nil {
		logrus.Error(err)
		return err
	}

	return nil
}

func getLatestTransactionInDB(DBClient *sql.DB, pair string) (transactionHistory, error) {
	latestBuy, err := getLatestTransactionInTable(DBClient, pair+"_buy_history")
	if err != nil {
		logrus.Error(err)
		return transactionHistory{}, err
	}
	latestSell, err := getLatestTransactionInTable(DBClient, pair+"_sell_history")
	if err != nil {
		logrus.Error(err)
		return transactionHistory{}, err
	}

	if latestBuy.timestamp > latestSell.timestamp {
		return latestBuy, nil
	}

	return latestSell, nil
}

func checkIfTransactionExistInDB(DBClient *sql.DB, pair string) (bool, error) {
	trans, err := getLatestTransactionInTable(DBClient, pair+"_buy_history")
	if err != nil {
		logrus.Error(err)
		return false, err
	} else if trans != (transactionHistory{}) {
		return true, nil
	}

	trans, err = getLatestTransactionInTable(DBClient, pair+"_sell_history")
	if err != nil {
		logrus.Error(err)
		return false, err
	} else if trans != (transactionHistory{}) {
		return true, nil
	}

	return false, nil
}

func createBotStatusTable(DBClient *sql.DB) error {
	return CreateTable(DBClient, "bot_status", "id SERIAL PRIMARY KEY, bot_status TEXT")
}

func dBGetTransactionByOrderID(DBClient *sql.DB, orderID int64, side string) (transactionHistory, error) {
	var transaction transactionHistory

	req := `select * from ethbusd_` + strings.ToLower(side) + `_history where order_id=$1;`

	err := DBClient.QueryRow(req, orderID).Scan(&transaction.id, &transaction.timestamp, &transaction.price, &transaction.quantity, &transaction.total,
		&transaction.pair, &transaction.orderID, &transaction.botTransaction, &transaction.side, &transaction.fee)

	return transaction, err
}

func updateCoin(DBClient *sql.DB, pair, status string) (string, error) {

	query := `UPDATE coins_to_monitor
	SET status=$1
	WHERE pair=$2
	RETURNING id;`

	id := 0
	pp.Println(query)
	err := DBClient.QueryRow(query, status, pair).Scan(&id)
	if err != nil {
		logrus.Error(err)
		return "", err
	}

	return pair + " updated", nil
}
