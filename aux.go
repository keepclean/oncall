package main

import (
	"log"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/table"
	"github.com/rickar/cal"
)

var cUK = cal.NewCalendar()
var cUS = cal.NewCalendar()
var cSG = cal.NewCalendar()

func init() {
	cal.AddBritishHolidays(cUK)
	cal.AddUsHolidays(cUS)
	// Singapore public holidays
	cSG.AddHoliday(
		cal.NewYear,
		cal.Holiday{Month: time.February, Day: 5}, // Chinese New Year
		cal.Holiday{Month: time.February, Day: 6}, // Chinese New Year
		cal.GoodFriday,
		cal.Holiday{Month: time.May, Day: 1},      // Labour Day
		cal.Holiday{Month: time.May, Day: 19},     // Vesak Day
		cal.Holiday{Month: time.May, Day: 20},     // Vesak Day Holiday
		cal.Holiday{Month: time.June, Day: 5},     // Hari Raya Puasa
		cal.Holiday{Month: time.August, Day: 9},   // National Day
		cal.Holiday{Month: time.August, Day: 11},  // Hari Raya Haji
		cal.Holiday{Month: time.August, Day: 12},  // Hari Raya Haji Holiday
		cal.Holiday{Month: time.October, Day: 27}, // Deepavali
		cal.Holiday{Month: time.October, Day: 28}, // Deepavali Holiday
		cal.Christmas,
	)
}

func token() (t string) {
	t, ok := os.LookupEnv("PAGERDUTY_API_TOKEN")
	if !ok {
		log.Fatalln("Environment variable PAGERDUTY_API_TOKEN must be set")
	}

	return
}

func convertTime(t, f string) (string, error) {
	if f == "" {
		f = "2006-01-02 15:04"
	}

	parsedTime, err := parseTime(t)
	if err != nil {
		return "", err
	}

	return parsedTime.Format(f), nil
}

func weekday(t string) (time.Weekday, error) {
	parsedTime, err := parseTime(t)
	if err != nil {
		return 0, err
	}

	return parsedTime.Weekday(), nil
}

func holidays(t string) ([]string, error) {
	var countries []string

	parsedTime, err := parseTime(t)
	if err != nil {
		return countries, err
	}

	if cUK.IsHoliday(parsedTime) {
		countries = append(countries, "UK")
	}
	if cUS.IsHoliday(parsedTime) {
		countries = append(countries, "US")
	}
	if cSG.IsHoliday(parsedTime) {
		countries = append(countries, "SG")
	}

	return countries, nil
}

func parseTime(t string) (pt time.Time, err error) {
	pt, err = time.Parse(time.RFC3339, t)
	return
}

var tableStyle string
var tableStyles = map[string]table.Style{
	"box":     table.StyleDefault,
	"rounded": table.StyleRounded,
	"colored": table.StyleColoredBright,
}

func printTable(data []table.Row, fields table.Row) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(tableStyles[tableStyle])
	t.AppendHeader(fields)
	t.AppendRows(data)
	t.Render()
}

func checkDate(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		log.Fatalln(err)
	}
	return t.Format("2006-01-02")
}

func workDays(s1, s2 string) uint {
	t1, err := time.Parse("2006-01-02", s1)
	if err != nil {
		log.Fatalln(err)
	}
	t2, err := time.Parse("2006-01-02", s2)
	if err != nil {
		log.Fatalln(err)
	}

	var n uint
	for {
		if t1.Weekday() != 0 && t1.Weekday() != 6 && !cUK.IsHoliday(t1) {
			n++
		}
		if t2.Equal(t1) {
			break
		}
		t1 = t1.AddDate(0, 0, 1)
	}

	return n
}

func storyPoints(offShiftDays uint) uint {
	// We know that SP are not about time,
	// but we need to start an estimation from some basic values.

	// One ideal work week it's 5 working days and 2 weekend days.
	// 5 working days give us 1 dedicated working day for sprint or 7 story points.
	// 1.4 it's a number of story points in each sprint day.

	// If engineers during a working week have one tactical day, then they
	// have 4 days for sprint and roughly 5 story points in sprint.

	// The less an engineers have days off shift, the less points they have.

	return uint(float32(offShiftDays) * 1.4)
}
