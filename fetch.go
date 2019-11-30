package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var apiHost = "https://api.pagerduty.com/"
var client = &http.Client{
	Timeout: time.Second * 10,
}

type pagerdutyScheduleResponse struct {
	PagerdutySchedule *pagerdutySchedule `json:"schedule"`
}

type pagerdutySchedule struct {
	FinalSchedule *finalSchedule `json:"final_schedule"`
	Oncall        *onCall        `json:"oncall"`
	Users         []*entryUser   `json:"users"`
}

type finalSchedule struct {
	RenderedScheduleEntries []*scheduleEntry `json:"rendered_schedule_entries"`
}

type scheduleEntry struct {
	Start     string     `json:"start"`
	End       string     `json:"end"`
	EntryUser *entryUser `json:"user"`
}

type onCall struct {
	EntryUser *entryUser `json:"user"`
}

type entryUser struct {
	ID   string `json:"id"`
	Name string `json:"summary"`
}

func getSchedule(shiftID, startdate, enddate, token string) (*pagerdutyScheduleResponse, error) {
	vals := make(url.Values)
	vals.Set("include_oncall", "true")
	if startdate != "" {
		vals.Set("since", startdate)
	}
	if enddate != "" {
		vals.Set("until", enddate)
	}
	u := fmt.Sprint(apiHost, "schedules/", shiftID, "?", vals.Encode())
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Accept", "application/vnd.pagerduty+json;version=2")
	req.Header.Set("Authorization", fmt.Sprintf("Token token=%s", token))

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("%s", res.Status)
	}
	defer res.Body.Close()

	var response pagerdutyScheduleResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}
