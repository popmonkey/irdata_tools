package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/popmonkey/irdata"
)

const resultCacheHours = 4 * 365 * 24 // 4 years ;)
const cacheHours = 1

type League struct {
	leagueId int
	ir       *irdata.Irdata
	db       *sql.DB
}

func NewLeague(ir *irdata.Irdata, leagueId int) *League {
	return &League{
		leagueId: leagueId,
		ir:       ir,
	}
}

func (l *League) processLeague() {
	l.ir.EnableCache("data/.ircache")
	l.ir.SetLogLevel(irdata.LogLevelInfo)
	l.OpenWriter()

	defer l.CloseWriter()

	// read league info
	data, err := l.ir.GetWithCache(fmt.Sprintf("/data/league/get?league_id=%d", l.leagueId), cacheHours)
	if err != nil {
		log.Panic(err)
	}

	var rawLeague map[string]interface{}

	err = json.Unmarshal(data, &rawLeague)
	if err != nil {
		log.Panic(err)
	}

	// drop this roster because we'll load that into another parquet
	delete(rawLeague, "roster")

	l.WriteParquet(rawLeague, "league")

	// read league roster
	data, err = l.ir.GetWithCache(fmt.Sprintf("/data/league/roster?league_id=%d", l.leagueId), cacheHours)
	if err != nil {
		log.Panic(err)
	}

	var rawRoster map[string]interface{}

	err = json.Unmarshal(data, &rawRoster)
	if err != nil {
		log.Panic(err)
	}

	l.WriteParquet(rawRoster["roster"], "roster")

	// read league seasons
	data, err = l.ir.GetWithCache(fmt.Sprintf("/data/league/seasons?league_id=%d&retired=true", l.leagueId), cacheHours)
	if err != nil {
		log.Panic(err)
	}

	var rawSeasons map[string]interface{}

	err = json.Unmarshal(data, &rawSeasons)
	if err != nil {
		log.Panic(err)
	}

	l.WriteParquet(rawSeasons["seasons"], "seasons")

	for _, s := range rawSeasons["seasons"].([]interface{}) {
		s := s.(map[string]interface{})

		l.processSeason(int(s["season_id"].(float64)))
	}

	l.MergeParquet("sessions-*", "sessions")
	l.MergeParquet("results-*", "results")
	l.MergeParquet("team-results-*", "team-results")
}

func (l *League) processSeason(seasonId int) {
	data, err := l.ir.GetWithCache(
		fmt.Sprintf("/data/league/season_sessions?league_id=%d&season_id=%d",
			l.leagueId, seasonId), cacheHours)
	if err != nil {
		log.Panic(err)
	}

	var rawSessions map[string]interface{}

	err = json.Unmarshal(data, &rawSessions)
	if err != nil {
		log.Panic(err)
	}

	// strip weather for now
	for _, s := range rawSessions["sessions"].([]interface{}) {
		s := s.(map[string]interface{})

		delete(s, "weather")
	}

	if len(rawSessions["sessions"].([]interface{})) == 0 {
		return
	}

	l.WriteParquet(rawSessions["sessions"], fmt.Sprintf("sessions-%d", seasonId))

	for _, s := range rawSessions["sessions"].([]interface{}) {
		s := s.(map[string]interface{})

		if s["has_results"].(bool) {
			if s["driver_changes"].(bool) {
				l.processSession("team-", s)
			} else {
				l.processSession("", s)
			}
		}
	}
}

func (l *League) processSession(sessionPrefix string, session map[string]interface{}) {
	subsessionId := int(session["subsession_id"].(float64))

	if l.SessionExists(sessionPrefix, subsessionId) {
		return
	}

	data, err := l.ir.GetWithCache(fmt.Sprintf("/data/results/get?subsession_id=%d", subsessionId), resultCacheHours)
	if err != nil {
		log.Panic(err)
	}

	var subsession map[string]interface{}

	err = json.Unmarshal(data, &subsession)
	if err != nil {
		log.Panic(err)
	}

	for _, s := range subsession["session_results"].([]interface{}) {
		s := s.(map[string]interface{})

		s["subsession_id"] = subsessionId
		l.WriteParquet(s, fmt.Sprintf("%sresults-%d_%d", sessionPrefix, subsessionId, int(s["simsession_number"].(float64))))
	}
}
