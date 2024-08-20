package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"sort"
	"time"

	"github.com/araddon/dateparse"
	"github.com/popmonkey/irdata"
)

var (
	ir *irdata.Irdata
)

func init() {
	ir = irdata.Open(context.Background())

	ir.SetLogLevel(irdata.LogLevelError)
}

func main() {
	var err error

	if len(os.Args) != 5 {
		fmt.Println("Usage: ban_check <keyfile> <credsfile> <member name> <date>")
		os.Exit(1)
	}

	var (
		keyFile    = os.Args[1]
		credsFile  = os.Args[2]
		searchTerm = os.Args[3]
		startDate  = os.Args[4]
	)

	// valiDate, lol
	startTime, err := dateparse.ParseLocal(startDate)
	if err != nil {
		log.Fatalf("invalid date: %s\n", startDate)
	}

	finishTime := startTime.Add(time.Duration(30*24) * time.Hour)
	if finishTime.After(time.Now()) {
		finishTime = time.Now()
	}

	_, err = os.Stat(credsFile)
	if err != nil {
		log.Panic("no creds, no data")
	}

	err = ir.AuthWithCredsFromFile(keyFile, credsFile)
	if err != nil {
		log.Panic(err)
	}

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
		log.Fatalf("no members found matching %s\n", searchTerm)
	}

	if resultCount > 1 {
		for index, result := range searchResults {
			r := result.(map[string]interface{})
			log.Printf("%d. %s [%d]", index, r["display_name"].(string), int(r["cust_id"].(float64)))
		}
		log.Fatal("be more specific (you can use the member id)...")
	}

	r := searchResults[0].(map[string]interface{})

	var (
		memberName = r["display_name"].(string)
		memberId   = int(r["cust_id"].(float64))
	)

	data, err = ir.Get(fmt.Sprintf("/data/member/get?cust_ids=%d", memberId))
	if err != nil {
		log.Panic(err)
	}

	var memberList map[string]interface{}

	err = json.Unmarshal(data, &memberList)
	if err != nil {
		log.Panic(err)
	}

	memberInfo := memberList["members"].([]interface{})[0].(map[string]interface{})

	memberSince, err := dateparse.ParseAny(memberInfo["member_since"].(string))
	if err != nil {
		log.Panic(err)
	}

	lastLogin, err := dateparse.ParseAny(memberInfo["last_login"].(string))
	if err != nil {
		log.Panic(err)
	}

	fmt.Printf("\n%[1]s [%[2]d] (joined: %[5]v, last: %[6]v)\n\twas member for %0.0[7]f days prior to %[3]v\n\tsearching until %[4]v\n\n",
		memberName,
		memberId,
		startTime.Format("2006-01-02 15:04 Z0700"),
		finishTime.Format("2006-01-02 15:04 Z0700"),
		memberSince.Format("2006-01-02"),
		lastLogin.Format("2006-01-02"),
		startTime.Sub(memberSince).Hours()/24.0,
	)

	uri := fmt.Sprintf("/data/results/search_series?cust_id=%d&start_range_begin=%s&start_range_end=%s",
		memberId, startTime.Format("2006-01-02T15:04Z"), finishTime.Format("2006-01-02T15:04Z"))

	data, err = ir.Get(uri)
	if err != nil {
		log.Panic(err)
	}

	var sessionsWrapper map[string]interface{}

	err = json.Unmarshal(data, &sessionsWrapper)
	if err != nil {
		log.Panic(err)
	}

	sessionsData, ok := sessionsWrapper["data"]
	if !ok {
		log.Panicf("[%s] %s\n", sessionsWrapper["error"], sessionsWrapper["message"])
	}

	var (
		current time.Time
		gaps    []struct {
			start    time.Time
			duration time.Duration
		}
	)

	current = startTime

	chunkData := sessionsData.(map[string]interface{})["_chunk_data"]
	if chunkData == nil {
		log.Fatalf("no sessions found between %v and %v\n", startTime, finishTime)
	}

	for _, session := range sessionsData.(map[string]interface{})["_chunk_data"].([]interface{}) {
		s := session.(map[string]interface{})
		sessionStartTime, err := dateparse.ParseAny(s["start_time"].(string))
		if err != nil {
			log.Panic(err)
		}

		if s["official_session"].(bool) && int(s["event_type"].(float64)) == 5 {
			fmt.Printf("%[4]s %[1]t %[2]s [%[3]d] : laps: %[6]d of %[7]d inc: %[5]d\n",
				s["official_session"].(bool),
				s["series_name"].(string),
				int(s["subsession_id"].(float64)),
				s["start_time"].(string),
				int(s["incidents"].(float64)),
				int(s["laps_complete"].(float64)),
				int(s["event_laps_complete"].(float64)),
			)

			gaps = append(gaps, struct {
				start    time.Time
				duration time.Duration
			}{
				start:    current,
				duration: sessionStartTime.Sub(current),
			})

			current = sessionStartTime
		}
	}

	gaps = append(gaps, struct {
		start    time.Time
		duration time.Duration
	}{
		start:    current,
		duration: finishTime.Sub(current),
	})

	sort.Slice(gaps, func(i, j int) bool {
		return gaps[i].duration.Hours() > gaps[j].duration.Hours()
	})

	fmt.Printf("\nLargest gap: %0.2[1]f days starting %[2]v\n", gaps[0].duration.Hours()/24.0, gaps[0].start)
}
