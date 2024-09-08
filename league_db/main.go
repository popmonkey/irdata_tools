package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/popmonkey/irdata"
)

var (
	ir *irdata.Irdata
)

const resultCacheHours = 4 * 365 * 24 // 4 years ;)

func init() {
	ir = irdata.Open(context.Background())

	ir.SetLogLevel(irdata.LogLevelError)

}

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

	processRoster(leagueId)

	processSeasons(leagueId)
}

func processRoster(leagueId int) {
	rosterUrl := fmt.Sprintf("data/league/roster?league_id=%d", leagueId)

	data, err := ir.GetWithCache(rosterUrl, time.Duration(1)*time.Hour)
	if err != nil {
		log.Panic(err)
	}

	type rosterT struct {
		Private bool      `json:"private_roster"`
		Roster  []driverT `json:"roster"`
	}

	var roster rosterT

	err = json.Unmarshal(data, &roster)
	if err != nil {
		log.Panic(err)
	}

	var rosterRaw map[string]interface{}

	err = json.Unmarshal(data, &rosterRaw)
	if err != nil {
		log.Panic(err)
	}

	writeRoster(roster.Roster, rosterRaw["roster"].([]interface{}))
}

func processSeasons(leagueId int) {
	data, err := ir.GetWithCache(fmt.Sprintf("/data/league/seasons?league_id=%d&retired=true", leagueId), time.Duration(1)*time.Hour)
	if err != nil {
		log.Panic(err)
	}

	log.Print(data)

	var league leagueT
	err = json.Unmarshal(data, &league)
	if err != nil {
		log.Panic(err)
	}

	for _, season := range league.Seasons {
		processSeason(leagueId, season)
	}

}

func processSeason(leagueId int, season seasonT) {
	log.Print(season.Season_Name)

	data, err := ir.GetWithCache(fmt.Sprintf("/data/league/season_sessions?league_id=%d&season_id=%d", leagueId, season.Season_Id), time.Duration(1)*time.Hour)
	if err != nil {
		log.Panic(err)
	}

	var sessions seasonSessionsT
	var sessionsRaw interface{}

	err = json.Unmarshal(data, &sessions)
	if err != nil {
		log.Panic(err)
	}

	err = json.Unmarshal(data, &sessionsRaw)
	if err != nil {
		log.Panic(err)
	}

	for _, session := range sessions.Sessions {
		processSeasonsSession(session)
	}
}

func processSeasonsSession(session sessionT) {
	log.Printf("%s @ %s (%s)", session.Launch_At, session.Track.Track_Name, session.Track.Config_Name)

	data, err := ir.GetWithCache(fmt.Sprintf("/data/results/get?subsession_id=%d", session.Subsession_Id), time.Duration(resultCacheHours)*time.Hour)
	if err != nil {
		log.Panic(err)
	}

	var subsession map[string]interface{}

	err = json.Unmarshal(data, &subsession)
	if err != nil {
		log.Panic(err)
	}

	if subsession["session_results"] == nil {
		return
	}
}
