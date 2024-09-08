package main

import (
	"log"
	"os"

	"github.com/parquet-go/parquet-go"
)

func writeRoster(roster []driverT, raw []interface{}) {
	f, err := os.OpenFile("roster.parquet", os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Panic(err)
	}

	writer := parquet.NewGenericWriter[driverT](f)

	defer writer.Close()

	_, err = writer.Write(roster)
	if err != nil {
		log.Panic(err)
	}

	rawWriter := parquet.NewGenericWriter[map[string]interface{}](f)

	defer rawWriter.Close()

	// _, err = rawWriter.Write(raw)
	// if err != nil {
	// 	log.Panic(err)
	// }
}
