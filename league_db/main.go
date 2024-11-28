package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/popmonkey/irdata"
)

/*
1. Get League info [/data/league/get?league_id=?]
2. Get League roster [/data/league/roster?league_id=?]
3. Get Seasons [/data/league/seasons?retired=true&league_id=?]
4.   Get Results [/data/league/season_sessions?league_id=&season	_id=]
*/

func main() {
	var err error

	if len(os.Args) != 4 {
		fmt.Println("Usage: stats <keyfile> <credsfile> <league id>")
		os.Exit(1)
	}

	var (
		keyFile   = os.Args[1]
		credsFile = os.Args[2]
	)

	leagueId, err := strconv.Atoi(os.Args[3])
	if err != nil {
		log.Panic(err)
	}

	var credsProvider irdata.CredsFromTerminal

	ir := irdata.Open(context.Background())
	ir.SetLogLevel(irdata.LogLevelError)

	_, err = os.Stat(credsFile)
	if err != nil {
		err = ir.AuthAndSaveProvidedCredsToFile(keyFile, credsFile, credsProvider)
	} else {
		err = ir.AuthWithCredsFromFile(keyFile, credsFile)
	}

	if err != nil {
		log.Panic(err)
	}

	ir.EnableCache(".cache")

	l := NewLeague(ir, leagueId)

	l.processLeague()
}
