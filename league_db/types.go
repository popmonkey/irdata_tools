package main

type driverT struct {
	Cust_Id             int64  `json:"cust_id"`
	Display_Name        string `json:"display_name"`
	Owner               bool   `json:"owner"`
	Admin               bool   `json:"admin"`
	League_Member_Since string `json:"league_member_since"`
	Car_Number          string `json:"car_number,omitempty"`
	Nick_Name           string `json:"nick_name,omitempty"`
}

type seasonT struct {
	Season_Id                     int64         `json:"season_id"`
	Season_Name                   string        `json:"season_name"`
	League_Id                     int64         `json:"league_id"`
	Active                        bool          `json:"active"`
	Hidden                        bool          `json:"hidden"`
	Points_System_Desc            string        `json:"points_system_desc"`
	Num_Drops                     int           `json:"num_drops"`
	No_Drops_On_Or_After_Race_Num int           `json:"no_drop_on_or_after_race_num"`
	Points_System_Name            string        `json:"points_system_name"`
	Points_Cars                   []interface{} `json:"points_cars"`
	Driver_Points_Car_Classes     []interface{} `json:"driver_points_car_classes"`
	TeamP_oints_Car_Classes       []interface{} `json:"team_points_car_classes"`
}

type leagueT struct {
	League_Id  int64     `json:"league_id"`
	Retired    bool      `json:"retired"`
	Success    bool      `json:"success"`
	Subscribed bool      `json:"subscribed"`
	Seasons    []seasonT `json:"seasons"`
}

type trackT struct {
	Track_Id    int64  `json:"track_id"`
	Track_Name  string `json:"track_name"`
	Config_Name string `json:"config_name"`
}

type carT struct {
	Car_Id         int64  `json:"car_id"`
	Car_Name       string `json:"car_name"`
	Car_Class_Id   int64  `json:"car_class_id"`
	Car_Class_Name string `json:"car_class_name"`
}

// to be finished
type trackStateT struct {
	Qualify_Grip_Compound int `json:"qualify_grip_compound"`
}

// to be finished
type weatherT struct {
	Weather_Summary map[string]interface{} `json:"weather_summary"`
	Weather_Url     string                 `json:"weather_url"`
}

type sessionT struct {
	Session_Id         int64 `json:"sessions_id"`
	Subsession_Id      int64 `json:"subsession_id"`
	League_Season_Id   int64 `json:"league_season_id"`
	League_Id          int64 `json:"league_id"`
	Private_Session_Id int64 `json:"private_session_id"`

	Driver_Changes  bool `json:"driver_changes"`
	Practice_Length int  `json:"practice_length"`
	Lone_Qualify    bool `json:"lone_qualify"`
	Qualify_Laps    int  `json:"qualify_laps"`
	Qualify_Length  int  `json:"qualify_length"`
	Race_Laps       int  `json:"race_laps"`
	Race_Length     int  `json:"race_length"`

	Launch_At        string `json:"launch_at"`
	Has_Results      bool   `json:"has_results"`
	Status           int    `json:"status"`
	Winner_Id        int64  `json:"winner_id"`
	Winner_Name      string `json:"winner_name"`
	Entry_Count      int    `json:"entry_count"`
	Team_Entry_Count int    `json:"team_entry_count"`

	Password_Protected bool  `json:"password_protected"`
	Pace_Car_Id        int64 `json:"pace_car_id,omitempty"`

	TrackState trackStateT `json:"track_state"`
	Track      trackT      `json:"track"`
	Weather    weatherT    `json:"weather"`
	Cars       []carT      `json:"cars"`
}

type seasonSessionsT struct {
	SeasonId int64      `json:"season_id"`
	LeagueId int64      `json:"league_id"`
	Success  bool       `json:"success"`
	Sessions []sessionT `json:"sessions"`
}
