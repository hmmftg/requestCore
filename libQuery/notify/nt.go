package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/lib/pq"
)

func Connect(connectionString string, channel chan string) *sql.DB {
	base, err := pq.NewConnector(connectionString)
	if err != nil {
		log.Fatal(err)
	}

	// Notice listner stores any notices in a slice to be returned after query is run
	connector := pq.ConnectorWithNoticeHandler(
		base,
		NotifyHandler,
	)

	return sql.OpenDB(connector)
}

type Results struct {
	allRows [][]string
	notices []string
}

var channel chan string

func NotifyHandler(notice *pq.Error) {
	channel <- notice.Message
}

func RunQuery(db *sql.DB, query string, myresults *Results) error {
	response, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}

	//var allRows [][]string
	var row []string
	row, err = response.Columns()
	if err != nil {
		fmt.Println("error getting column names")
		return err
	}
	myresults.allRows = append(myresults.allRows, row)

	raw := make([][]byte, len(row))       // byte slice is one column in the returned row. The slice of byte slice is to store multiple columns in one slice
	dest := make([]interface{}, len(row)) // slice of interface to insert unknown number of returned columns

	// We cannot scan directly into a slice of bytes and need to use an empty inteface to scan into
	// This assigns the address of the i th index to dest so when we scan into dest, raw slice is populated with the data we need to work on
	for i := range dest {
		dest[i] = &raw[i] // Use pointers to byte slices for Scan
	}

	for response.Next() {
		// scan row into the slice of interface which holds the addresses of raw variable
		err := response.Scan(dest...)
		if err != nil {
			fmt.Println("unable to scan row")
			break
		}

		row := make([]string, len(myresults.allRows[0]))
		for i, v := range raw { // loop through slice of byte slice
			if v == nil {
				row[i] = "" // nil object returned from db. cannot have nil string
			} else {
				row[i] = string(v)
			}
		}
		emptyColumns := 0
		for _, field := range row {
			if len(field) == 0 {
				emptyColumns++
			}
		}
		if emptyColumns == len(myresults.allRows[0]) {
			break
		}
		myresults.allRows = append(myresults.allRows, row)
	}
	return nil
}

func SendNotifs(db *sql.DB, notifs string) {
	result, err := db.Exec(notifs)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("execute notify result:", result)
}

func main() {
	sqlFileContents, err := os.ReadFile("/home/hamid/Projects/PCBO/Refah/backend/requestCore/requestCore/libQuery/notify/f.sql")
	if err != nil {
		log.Fatal(err)
	}
	channel = make(chan string)

	connectionString := "postgres://pcbo:pcbo@10.15.1.61:9254/test"
	db := Connect(connectionString, channel)
	statements := strings.Split(string(sqlFileContents), ";")
	var myresults Results
	err = RunQuery(db, statements[0], &myresults)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("query 1:", myresults)

	go SendNotifs(db, strings.Join(statements[1:6], ";")+";")

NotifyWaitLoop:
	for {
		select {
		case notify := <-channel:
			myresults.notices = append(myresults.notices, notify)
		case <-time.After(1 * time.Second):
			if len(myresults.notices) == len(myresults.allRows)-1 /*-1 is for columns which you have stored in query*/ {
				fmt.Println("all notifs are received")
				break NotifyWaitLoop
			}
		}
	}

	fmt.Println("notify result:", myresults)
	err = RunQuery(db, statements[7], &myresults)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("query 2:", myresults)
}
