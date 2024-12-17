package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/popmonkey/irdata"
)

const resultCacheTTL = time.Duration(4*365*24) * time.Hour // 4 years ;)
const cacheTTL = time.Duration(1) * time.Hour

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

	l.ConvertParquetToJson("league")
	l.ConvertParquetToJson("roster")
	l.ConvertParquetToJson("seasons")
	l.ConvertParquetToJson("sessions")
	l.ConvertParquetToJson("results")
	l.ConvertParquetToJson("team-results")
	l.ConvertParquetToJson("lap_data")

	defer l.CloseWriter()

	// read league info
	data, err := l.ir.GetWithCache(fmt.Sprintf("/data/league/get?league_id=%d", l.leagueId), cacheTTL)
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

	l.WriteJson(rawLeague, "league")

	// read league roster
	data, err = l.ir.GetWithCache(fmt.Sprintf("/data/league/roster?league_id=%d", l.leagueId), cacheTTL)
	if err != nil {
		log.Panic(err)
	}

	var rawRoster map[string]interface{}

	err = json.Unmarshal(data, &rawRoster)
	if err != nil {
		log.Panic(err)
	}

	l.WriteJson(rawRoster["roster"], "roster")

	// read league seasons
	data, err = l.ir.GetWithCache(fmt.Sprintf("/data/league/seasons?league_id=%d&retired=true", l.leagueId), cacheTTL)
	if err != nil {
		log.Panic(err)
	}

	var rawSeasons map[string]interface{}

	err = json.Unmarshal(data, &rawSeasons)
	if err != nil {
		log.Panic(err)
	}

	l.WriteJson(rawSeasons["seasons"], "seasons")

	for _, s := range rawSeasons["seasons"].([]interface{}) {
		s := s.(map[string]interface{})

		l.processSeason(int(s["season_id"].(float64)))
	}

	l.MergeJson("sessions-*", "sessions")
	l.MergeJson("results-*", "results")
	l.MergeJson("team-results-*", "team-results")
	l.MergeJson("lap_data-*", "lap_data")

	l.ConvertJsonToParquet("league")
	l.ConvertJsonToParquet("roster")
	l.ConvertJsonToParquet("seasons")
	l.ConvertJsonToParquet("sessions")
	l.ConvertJsonToParquet("results")
	l.ConvertJsonToParquet("team-results")
	l.ConvertJsonToParquet("lap_data")
}

func (l *League) processSeason(seasonId int) {
	data, err := l.ir.GetWithCache(
		fmt.Sprintf("/data/league/season_sessions?league_id=%d&season_id=%d",
			l.leagueId, seasonId), cacheTTL)
	if err != nil {
		log.Panic(err)
	}

	var rawSessions map[string]interface{}

	err = json.Unmarshal(data, &rawSessions)
	if err != nil {
		log.Panic(err)
	}

	l.WriteJson(rawSessions["sessions"], fmt.Sprintf("sessions-%d", seasonId))

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

	l.MergeJson("sessions-*", "sessions")
	l.MergeJson("results-*", "results")
	l.MergeJson("team-results-*", "team-results")
}

func (l *League) processSession(sessionPrefix string, session map[string]interface{}) {
	subsessionId := int(session["subsession_id"].(float64))

	if l.SessionExists(sessionPrefix, subsessionId) {
		return
	}

	data, err := l.ir.GetWithCache(fmt.Sprintf("/data/results/get?subsession_id=%d", subsessionId), resultCacheTTL)
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
		simsessionNumber := int(s["simsession_number"].(float64))

		for _, r := range s["results"].([]interface{}) {
			r := r.(map[string]interface{})

			var lapDataParams string
			var lapperId int

			if sessionPrefix == "" {
				lapperId = int(r["cust_id"].(float64))
				lapDataParams = fmt.Sprintf("cust_id=%d", lapperId)
			} else {
				lapperId = int(r["team_id"].(float64))
				lapDataParams = fmt.Sprintf("team_id=%d", lapperId)
			}

			data, err = l.ir.GetWithCache(fmt.Sprintf("/data/results/lap_data?subsession_id=%d&simsession_number=%d&%s",
				subsessionId, simsessionNumber, lapDataParams), resultCacheTTL)
			if err != nil {
				log.Panic(err)
			}

			var laps map[string]interface{}

			err = json.Unmarshal(data, &laps)
			if err != nil {
				log.Panic(err)
			}

			laps["events"] = laps["_chunk_data"]

			delete(laps, "chunk_info")
			delete(laps, "_chunk_data")

			l.WriteJson(laps, fmt.Sprintf("lap_data-%d_%d_%d", subsessionId, simsessionNumber, lapperId))
		}

		l.WriteJson(s, fmt.Sprintf("%sresults-%d_%d", sessionPrefix, subsessionId, simsessionNumber))

		l.MergeJson("lap_data-*", "lap_data")
	}
}
