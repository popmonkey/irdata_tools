package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/popmonkey/irdata"
)

// ./irfetch -c -v ~/irs.key ~/ir.creds "/data/league/seasons?league_id=8093"
// ./irfetch -c -v ~/irs.key ~/ir.creds "/data/league/season_sessions?league_id=8093&season_id=108390"
// ./irfetch -c -v ~/irs.key ~/ir.creds "/data/results/get?subsession_id=69983822"

var (
	ir            *irdata.Irdata
	credsProvider irdata.CredsFromTerminal
	db            *sql.DB
)

const resultCacheHours = 4 * 365 * 24

func init() {
	ir = irdata.Open(context.Background())

	ir.SetLogLevel(irdata.LogLevelDebug)

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
		leagueId  = os.Args[3]
	)

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

	leagueIdNum, err := strconv.Atoi(leagueId)
	if err != nil {
		log.Panic(err)
	}

	db, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Panic(err)
	}
	defer db.Close()

	createDriverStmt := `
		CREATE TABLE driver (
			name VARCHAR NOT NULL PRIMARY KEY,
			active VARCHAR DEFAULT 'false',
			races INTEGER,
			laps INTEGER,
			incident_points INTEGER,
			incident_offtrack_count INTEGER,
			incident_controlloss_count INTEGER,
			incident_carcontact_count INTEGER,
			incident_contact_count INTEGER,
			blackflag_count INTEGER
		)
	`

	_, err = db.Exec(createDriverStmt)
	if err != nil {
		log.Panic(err)
	}

	processLeague(int64(leagueIdNum))
}

func processLeague(leagueId int64) {
	data, err := ir.GetWithCache(fmt.Sprintf("/data/league/seasons?league_id=%d&retired=true", leagueId), time.Duration(1)*time.Hour)
	if err != nil {
		log.Panic(err)
	}

	var league map[string]interface{}

	err = json.Unmarshal(data, &league)
	if err != nil {
		log.Panic(err)
	}

	for _, season := range league["seasons"].([]interface{}) {
		processSeason(leagueId, season.(map[string]interface{}))
	}

	selectDriversSql := `
		SELECT
		    name,
			active,
			races,
		    laps,
			incident_points,
			incident_offtrack_count,
			incident_controlloss_count,
			incident_carcontact_count,
			incident_contact_count,
			blackflag_count
		FROM driver
	`

	rows, err := db.Query(selectDriversSql)
	if err != nil {
		log.Panic(err)
	}

	fmt.Printf("Driver,Active,Races,Laps,Inc,Offtracks,ControlLosses,CarContacts,Contacts,BlackFlags\n")

	for rows.Next() {
		var (
			name                       sql.NullString
			active                     sql.NullString
			races                      sql.NullInt64
			laps                       sql.NullInt64
			incident_points            sql.NullInt64
			incident_offtrack_count    sql.NullInt64
			incident_controlloss_count sql.NullInt64
			incident_carcontact_count  sql.NullInt64
			incident_contact_count     sql.NullInt64
			blackflag_count            sql.NullInt64
		)

		err := rows.Scan(
			&name,
			&active,
			&races,
			&laps,
			&incident_points,
			&incident_offtrack_count,
			&incident_controlloss_count,
			&incident_carcontact_count,
			&incident_contact_count,
			&blackflag_count,
		)
		if err != nil {
			log.Panic(err)
		}

		fmt.Printf("%s,%s,%d,%d,%d,%d,%d,%d,%d,%d\n",
			name.String,
			active.String,
			races.Int64,
			laps.Int64,
			incident_points.Int64,
			incident_offtrack_count.Int64,
			incident_controlloss_count.Int64,
			incident_carcontact_count.Int64,
			incident_contact_count.Int64,
			blackflag_count.Int64,
			// incident_offtrack_count.Int64*1+
			// 	incident_controlloss_count.Int64*2+
			// 	incident_carcontact_count.Int64*4,
		)
	}
}

// func ratio(driver *driverT) float64 {
// 	if driver.laps == 0 {
// 		return 0.0
// 	}

// 	return float64(driver.incidents) / float64(driver.laps)
// }

func processSeason(leagueId int64, season map[string]interface{}) {
	id := int64(season["season_id"].(float64))
	name := season["season_name"].(string)

	log.Print(name)

	data, err := ir.GetWithCache(fmt.Sprintf("/data/league/season_sessions?league_id=%d&season_id=%d", leagueId, id), time.Duration(1)*time.Hour)
	if err != nil {
		log.Panic(err)
	}

	var sessions map[string]interface{}

	err = json.Unmarshal(data, &sessions)
	if err != nil {
		log.Panic(err)
	}

	activeSeason := season["active"].(bool)

	for _, s := range sessions["sessions"].([]interface{}) {
		session := s.(map[string]interface{})
		if session["has_results"].(bool) {
			processSession(session, activeSeason)
		}
	}
}

func processSession(seasonSession map[string]interface{}, activeSeason bool) {
	if seasonSession["subsession_id"] == nil {
		return
	}

	id := int64(seasonSession["subsession_id"].(float64))

	data, err := ir.GetWithCache(fmt.Sprintf("/data/results/get?subsession_id=%d", id), time.Duration(resultCacheHours)*time.Hour)
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

				subsession_id := int(subsession["subsession_id"].(float64))
				simsession_number := int(sr["simsession_number"].(float64))

				if tr["driver_results"] == nil {
					processDriver(tr, subsession_id, simsession_number, activeSeason)
				} else {
					for _, driverResult := range tr["driver_results"].([]interface{}) {
						dr := driverResult.(map[string]interface{})

						processDriver(dr, subsession_id, simsession_number, activeSeason)
					}
				}
			}
		}
	}
}

func processDriver(dr map[string]interface{}, subsession_id int, simsession_number int, activeSeason bool) {
	if dr["ai"].(bool) {
		log.Printf("%s is an AI Driver - skipping", dr["display_name"].(string))
		return
	}

	type incidentCounterT struct {
		offtrack    int
		contact     int
		carContact  int
		lostControl int
		blackFlag   int
	}

	var lapDataParams string

	if dr["team_id"] == nil {
		lapDataParams = fmt.Sprintf("cust_id=%d", int(dr["cust_id"].(float64)))
	} else {
		lapDataParams = fmt.Sprintf("team_id=%d", int(dr["team_id"].(float64)))
	}

	data, err := ir.GetWithCache(
		fmt.Sprintf(
			"/data/results/lap_data?subsession_id=%d&simsession_number=%d&%s",
			subsession_id, simsession_number, lapDataParams),
		time.Duration(resultCacheHours)*time.Hour,
	)
	if err != nil {
		log.Panic(err)
	}

	var lapData map[string]interface{}

	err = json.Unmarshal(data, &lapData)
	if err != nil {
		log.Panic(err)
	}

	incidentCollector := incidentCounterT{
		offtrack:    0,
		contact:     0,
		carContact:  0,
		lostControl: 0,
		blackFlag:   0,
	}

	var incidentLog []string

	if lapData[irdata.ChunkDataKey] != nil {
		for _, le := range lapData[irdata.ChunkDataKey].([]interface{}) {
			lapEvent := le.(map[string]interface{})

			if lapEvent["incident"].(bool) {
				for _, inc := range lapEvent["lap_events"].([]interface{}) {
					switch inc.(string) {
					case "off track":
						incidentLog = append(incidentLog, "offtrack")
						incidentCollector.offtrack++
					case "contact":
						incidentLog = append(incidentLog, "contact")
						incidentCollector.contact++
					case "car contact":
						incidentLog = append(incidentLog, "car contact")
						incidentCollector.carContact++
					case "lost control":
						incidentLog = append(incidentLog, "lost control")
						incidentCollector.lostControl++
					case "black flag":
						incidentLog = append(incidentLog, "black flag")
						incidentCollector.blackFlag++
					default:
						incidentLog = append(incidentLog, fmt.Sprintf("(unknown : %s)", inc.(string)))
					}
				}
			}
		}
	}

	log.Printf("incident log: [%s]", strings.Join(incidentLog, ", "))

	name := dr["display_name"].(string)
	laps := int(dr["laps_complete"].(float64))
	incidentPoints := int(dr["incidents"].(float64))

	log.Printf("\t%s: laps: %d, incidents %d [%v]", name, laps, incidentPoints, incidentCollector)

	selectDriverStmt := `
		SELECT
			active,
		    races,
		    laps,
		    incident_points,
			incident_offtrack_count,
			incident_controlloss_count,
			incident_carcontact_count,
			incident_contact_count,
			blackflag_count
		FROM driver WHERE name=?
	`

	var (
		priorActive         string
		priorRaces          int
		priorLaps           int
		priorIncidentPoints int
		priorIncidentCounts incidentCounterT
	)

	err = db.QueryRow(selectDriverStmt, name).Scan(
		&priorActive,
		&priorRaces,
		&priorLaps,
		&priorIncidentPoints,
		&priorIncidentCounts.offtrack,
		&priorIncidentCounts.lostControl,
		&priorIncidentCounts.carContact,
		&priorIncidentCounts.contact,
		&priorIncidentCounts.blackFlag,
	)
	if err == nil {
		updateDriverStmt := `
			UPDATE driver SET
			    races=?,
			    laps=?,
			    incident_points=?,
			    incident_offtrack_count=?,
			    incident_controlloss_count=?,
			    incident_carcontact_count=?,
				incident_contact_count=?,
				blackflag_count=?
            WHERE name=?
		`

		_, err = db.Exec(updateDriverStmt,
			priorRaces+1,
			priorLaps+laps,
			priorIncidentPoints+incidentPoints,
			priorIncidentCounts.offtrack+incidentCollector.offtrack,
			priorIncidentCounts.lostControl+incidentCollector.lostControl,
			priorIncidentCounts.carContact+incidentCollector.carContact,
			priorIncidentCounts.contact+incidentCollector.contact,
			priorIncidentCounts.blackFlag+incidentCollector.blackFlag,
			name)
		if err != nil {
			log.Panic(err)
		}

	} else if errors.Is(err, sql.ErrNoRows) {
		insertDriverStmt := `
			INSERT INTO driver
			    (name, races, laps, incident_points, incident_offtrack_count, incident_controlloss_count, incident_carcontact_count, incident_contact_count, blackflag_count)
			VALUES (?, 1, ?, ?, ?, ?, ?, ?, ?)
		`

		_, err = db.Exec(insertDriverStmt, name, laps,
			incidentPoints,
			incidentCollector.offtrack,
			incidentCollector.lostControl,
			incidentCollector.carContact,
			incidentCollector.contact,
			incidentCollector.blackFlag,
		)
		if err != nil {
			log.Panic(err)
		}
	} else {
		log.Panic(err)
	}

	if priorActive == "false" && activeSeason {
		updateActiveStmt := `
			UPDATE driver SET active='true' WHERE name=?`

		_, err = db.Exec(updateActiveStmt, name)
		if err != nil {
			log.Panic(err)
		}
	}
}
