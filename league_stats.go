package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/popmonkey/irdata"
)

// ./irfetch -c -v ~/irs.key ~/ir.creds "/data/league/seasons?league_id=8093"
// ./irfetch -c -v ~/irs.key ~/ir.creds "/data/league/season_sessions?league_id=8093&season_id=108390"
// ./irfetch -c -v ~/irs.key ~/ir.creds "/data/results/get?subsession_id=69983822"

var (
	ir            *irdata.Irdata
	credsProvider irdata.CredsFromTerminal
)

func init() {
	ir = irdata.Open(context.Background())

	ir.SetLogLevel(irdata.LogLevelDebug)
}

func main() {
	if len(os.Args) != 5 {
		fmt.Println("Usage: stats <keyfile> <credsfile> <league tag> <league id>")
		os.Exit(1)
	}

	var (
		keyFile   = os.Args[1]
		credsFile = os.Args[2]
		leagueTag = os.Args[3]
		leagueId  = os.Args[4]
	)

	_, err := os.Stat(credsFile)
	if err != nil {
		err = ir.AuthAndSaveProvidedCredsToFile(keyFile, credsFile, credsProvider)
	} else {
		err = ir.AuthWithCredsFromFile(keyFile, credsFile)
	}

	if err != nil {
		log.Panic(err)
	}

	ir.EnableCache(".cache")

	leagueIdNum, err := strconv.Atoi(leagueId)
	if err != nil {
		log.Panic(err)
	}

	processLeague(leagueTag, int64(leagueIdNum))
}

type driverT struct {
	laps      int
	incidents int
}

type driversT map[string]*driverT

func processLeague(_ string, leagueId int64) {
	data, err := ir.GetWithCache(fmt.Sprintf("/data/league/seasons?league_id=%d", leagueId), time.Duration(12)*time.Hour)
	if err != nil {
		log.Panic(err)
	}

	var league map[string]interface{}

	err = json.Unmarshal(data, &league)
	if err != nil {
		log.Panic(err)
	}

	drivers := make(map[string]*driverT)

	for _, season := range league["seasons"].([]interface{}) {
		processSeason(drivers, leagueId, season.(map[string]interface{}))
	}

	driverNames := make([]string, 0, len(drivers))

	for name := range drivers {
		driverNames = append(driverNames, name)
	}

	sort.SliceStable(driverNames, func(i, j int) bool {
		return ratio(drivers[driverNames[i]]) > ratio(drivers[driverNames[j]])
	})

	fmt.Printf("Driver,Laps,Incidents,Ratio\n")

	for _, name := range driverNames {
		driver := drivers[name]
		fmt.Printf("%s,%d,%d,%.4f\n",
			name,
			driver.laps,
			driver.incidents,
			ratio(driver),
		)
	}
}

func ratio(driver *driverT) float64 {
	if driver.laps == 0 {
		return 0.0
	}

	return float64(driver.incidents) / float64(driver.laps)
}

func processSeason(drivers driversT, leagueId int64, season map[string]interface{}) {
	id := int64(season["season_id"].(float64))
	name := season["season_name"].(string)

	log.Print(name)

	data, err := ir.GetWithCache(fmt.Sprintf("/data/league/season_sessions?league_id=%d&season_id=%d", leagueId, id), time.Duration(12)*time.Hour)
	if err != nil {
		log.Panic(err)
	}

	var sessions map[string]interface{}

	err = json.Unmarshal(data, &sessions)
	if err != nil {
		log.Panic(err)
	}

	for _, session := range sessions["sessions"].([]interface{}) {
		processSession(drivers, session.(map[string]interface{}))
	}

}

func processSession(drivers driversT, seasonSession map[string]interface{}) {
	if seasonSession["subsession_id"] == nil {
		return
	}

	id := int64(seasonSession["subsession_id"].(float64))

	data, err := ir.GetWithCache(fmt.Sprintf("/data/results/get?subsession_id=%d", id), time.Duration(12)*time.Hour)
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

	for _, subsessionResult := range subsession["session_results"].([]interface{}) {
		sr := subsessionResult.(map[string]interface{})

		if sr["simsession_type_name"] == "Race" {
			track := subsession["track"].(map[string]interface{})
			log.Printf("%s, Week %d [%s]", subsession["league_season_name"], int(subsession["race_week_num"].(float64))+1, track["track_name"])
			for _, teamResult := range sr["results"].([]interface{}) {
				tr := teamResult.(map[string]interface{})

				if tr["driver_results"] == nil {
					processDriver(drivers, tr)
				} else {
					for _, driverResult := range tr["driver_results"].([]interface{}) {
						dr := driverResult.(map[string]interface{})

						processDriver(drivers, dr)
					}
				}
			}
		}
	}
}

func processDriver(drivers driversT, dr map[string]interface{}) {
	name := dr["display_name"].(string)
	laps := int(dr["laps_complete"].(float64))
	incidents := int(dr["incidents"].(float64))

	log.Printf("\t%s: laps: %d, incidents %d", name, laps, incidents)

	d, exists := drivers[name]
	if exists {
		d.laps += laps
		d.incidents += incidents
	} else {
		(drivers)[name] = &driverT{laps: laps, incidents: incidents}
	}
}
