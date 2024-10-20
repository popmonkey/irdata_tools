package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/popmonkey/irdata"
)

var ir *irdata.Irdata

func init() {
	ir = irdata.Open(context.Background())

	ir.SetLogLevel(irdata.LogLevelError)
}

func main() {
	var credsProvider irdata.CredsFromTerminal

	defer ir.Close()

	err := ir.AuthWithProvideCreds(credsProvider)
	if err != nil {
		log.Fatal("Failed to auth, check email and password")
	}

	err = ir.EnableCache(".racing_history_cache")
	if err != nil {
		log.Panic(err)
	}

	var memberId int

	for {
		fmt.Print("\niRacing name or member ID: ")

		inputReader := bufio.NewReader(os.Stdin)
		searchTerm, err := inputReader.ReadString('\n')
		if err != nil {
			log.Panic(err)
		}

		searchTerm = strings.TrimSuffix(searchTerm, "\n")

		// find user
		data, err := ir.Get(fmt.Sprintf("/data/lookup/drivers?search_term=%s", url.QueryEscape(searchTerm)))
		if err != nil {
			log.Panic(err)
		}

		var searchResults []interface{}

		err = json.Unmarshal(data, &searchResults)
		if err != nil {
			log.Panic(err)
		}

		resultCount := len(searchResults)

		if resultCount == 0 {
			log.Printf("no members found matching %s\n\n", searchTerm)
			continue
		}

		if resultCount > 1 {
			for index, result := range searchResults {
				r := result.(map[string]interface{})
				fmt.Printf("%d. %s [%d]\n", index, r["display_name"].(string), int(r["cust_id"].(float64)))
			}
			fmt.Printf("\n\nBe more specific (you can use the member id)\n\n")
			continue
		}

		memberId = int(searchResults[0].(map[string]interface{})["cust_id"].(float64))
		break
	}

	// get member info
	data, err := ir.GetWithCache(fmt.Sprintf("/data/member/get?cust_ids=%d", memberId), time.Duration(15)*time.Minute)
	if err != nil {
		log.Panic(err)
	}

	var membersContainer map[string]interface{}

	err = json.Unmarshal(data, &membersContainer)
	if err != nil {
		log.Panic(err)
	}

	members := membersContainer["members"].([]interface{})
	if len(members) == 0 {
		fmt.Printf("Member %d was not found\n", memberId)
		os.Exit(1)
	}

	member := membersContainer["members"].([]interface{})[0].(map[string]interface{})

	startTime, err := dateparse.ParseAny(member["member_since"].(string))
	if err != nil {
		log.Panic(err)
	}

	fmt.Printf("Member: %s\nSince: %s\n\n",
		member["display_name"].(string),
		startTime.String())

	fmt.Printf("Time,Series,Car,Track,StartPosition,FinishPosition,Incidents,ResultsUrl\n")

	for {
		finishTime := startTime.Add(time.Duration(90*24)*time.Hour - time.Duration(1)*time.Minute)

		cacheDuration := time.Duration(15) * time.Minute

		if finishTime.Before(time.Now()) {
			cacheDuration = time.Duration(365*24) * time.Hour
		}

		data, err = ir.GetWithCache(
			fmt.Sprintf("/data/results/search_series?cust_id=%d&start_range_begin=%s&start_range_end=%s&event_types=5",
				memberId, startTime.Format("2006-01-02T15:04Z"), finishTime.Format("2006-01-02T15:04Z")),
			cacheDuration)
		if err != nil {
			log.Panic(err)
		}

		type sessionT map[string]interface{}

		var sessionContainer sessionT

		err = json.Unmarshal(data, &sessionContainer)
		if err != nil {
			log.Panic(err)
		}

		if sessionContainer["error"] != nil {
			log.Printf("Error in data: %s [%s]", sessionContainer["message"], sessionContainer["error"])
		} else {
			chunkData := sessionContainer["data"].(map[string]interface{})["_chunk_data"]

			if chunkData != nil {
				for _, session := range chunkData.([]interface{}) {
					session := session.(map[string]interface{})
					trackContainer := session["track"].(map[string]interface{})
					track := fmt.Sprintf("%s (%s)", trackContainer["track_name"], trackContainer["config_name"])

					fmt.Printf("%s,%s,%s,%s,%d,%d,%d,%s\n",
						session["start_time"],
						session["series_short_name"],
						session["car_name"],
						track,
						int(session["starting_position_in_class"].(float64))+1,
						int(session["finish_position_in_class"].(float64))+1,
						int(session["incidents"].(float64)),
						fmt.Sprintf("https://members-ng.iracing.com/racing/results-stats/results?subsessionid=%d",
							int(session["subsession_id"].(float64))),
					)
				}
			}
		}

		startTime = finishTime.Add(time.Duration(1) * time.Minute)

		if startTime.After(time.Now()) {
			break
		}
	}

	log.Println("DONE")
}
