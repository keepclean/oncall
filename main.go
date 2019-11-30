package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/table"
	cli "gopkg.in/urfave/cli.v1"
)

func main() {
	shifts := map[string]string{}
	shiftsKeys := make([]string, 0, len(shifts))
	for k := range shifts {
		shiftsKeys = append(shiftsKeys, k)
	}

	app := cli.NewApp()
	app.Name = "oncall"
	app.Usage = "Information about oncall shift"
	app.Version = "0.08"
	t := time.Now()
	flags := []cli.Flag{
		cli.StringFlag{
			Name:  "shift",
			Usage: strings.Join(shiftsKeys, " | "),
		},
		cli.StringFlag{
			Name:  "start",
			Value: t.Format("2006-01-02"),
			Usage: "start date",
		},
		cli.StringFlag{
			Name:  "end",
			Value: t.AddDate(0, 0, 7).Format("2006-01-02"),
			Usage: "end date",
		},
		cli.StringFlag{
			Name:        "table-style",
			Value:       "rounded",
			Usage:       "rounded, box, colored",
			Destination: &tableStyle,
		},
	}
	app.Commands = []cli.Command{
		{
			Name:  "schedule",
			Usage: "Oncall schedule information",
			Flags: flags,
			Action: func(c *cli.Context) error {
				shift := c.String("shift")
				shiftID := shifts[shift]
				startdate := checkDate(c.String("start"))
				enddate := checkDate(c.String("end"))

				shiftSchedule(shift, shiftID, startdate, enddate, token())

				return nil
			},
		},
		{
			Name:  "report",
			Usage: "Generates report",
			Flags: flags,
			Action: func(c *cli.Context) error {
				shift := c.String("shift")
				shiftID := shifts[shift]
				startdate := checkDate(c.String("start"))
				enddate := checkDate(c.String("end"))

				shiftReport(shift, shiftID, startdate, enddate, token())

				return nil
			},
		},
		{
			Name:  "now",
			Usage: "List currently oncall",
			Flags: flags[len(flags)-1:],
			Action: func(c *cli.Context) error {
				shiftNow(shifts, token())

				return nil
			},
		},
		{
			Name:  "roster",
			Usage: "Shows roster for all known shifts",
			Flags: flags[1:],
			Action: func(c *cli.Context) error {
				startdate := checkDate(c.String("start"))
				enddate := checkDate(c.String("end"))

				shiftsRoster(shifts, startdate, enddate, token())

				return nil
			},
		},
		{
			Name:  "user",
			Usage: "Oncall schedule for user",
			Flags: append(flags[1:], cli.StringFlag{
				Name:  "name",
				Usage: "Firstname Lastname or Firstname or Lastname",
			}),
			Action: func(c *cli.Context) error {
				startdate := checkDate(c.String("start"))
				enddate := checkDate(c.String("end"))
				name := c.String("name")
				if name == "" {
					return cli.NewExitError("Please specify a user name", 1)
				}

				shiftsUser(shifts, name, startdate, enddate, token())

				return nil
			},
		},
		{
			Name:  "sprint",
			Usage: "Count and show story points for sprint between dates",
			Flags: flags[1:],
			Action: func(c *cli.Context) error {
				startdate := checkDate(c.String("start"))
				enddate := checkDate(c.String("end"))

				sprintPoints(shifts, startdate, enddate, token())

				return nil
			},
		},
		{
			Name:  "ops-roster",
			Usage: "lhr sre roster",
			Flags: flags[1:],
			Action: func(c *cli.Context) error {
				startdate := checkDate(c.String("start"))
				enddate := checkDate(c.String("end"))
				shift := map[string]string{}

				OpsRoster(shift, startdate, enddate, token())

				return nil
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func shiftSchedule(shift, shiftID, startdate, enddate, token string) {
	schedule, err := getSchedule(shiftID, startdate, enddate, token)
	if err != nil {
		log.Fatalf("Failed to get schedule: %v", err)
	}

	var data []table.Row
	for _, entry := range schedule.PagerdutySchedule.FinalSchedule.RenderedScheduleEntries {
		start, err := convertTime(entry.Start, "")
		if err != nil {
			start = entry.Start
		}
		day, _ := weekday(entry.Start)
		holiday, _ := holidays(entry.Start)

		data = append(data, table.Row{start, day.String(), entry.EntryUser.Name, shift, strings.Join(holiday, ", ")})
	}

	fields := table.Row{"START", "DAY", "ENGINEER", "SHIFT", "HOLIDAY"}
	printTable(data, fields)
}

func shiftReport(shift, shiftID, startdate, enddate, token string) {
	schedule, err := getSchedule(shiftID, startdate, enddate, token)
	if err != nil {
		log.Fatalf("Failed to get schedule: %v", err)
	}

	parsedSchedule := make(map[string]map[string]uint)
	for _, entry := range schedule.PagerdutySchedule.FinalSchedule.RenderedScheduleEntries {
		if parsedSchedule[entry.EntryUser.Name] == nil {
			parsedSchedule[entry.EntryUser.Name] = map[string]uint{}
		}

		parsedSchedule[entry.EntryUser.Name]["oncall"]++
		day, _ := weekday(entry.Start)
		if day == 0 || day == 6 {
			parsedSchedule[entry.EntryUser.Name]["weekends"]++
		}

		holiday, _ := holidays(entry.Start)
		if len(holiday) > 0 {
			parsedSchedule[entry.EntryUser.Name]["holidays"]++
		}
	}

	var data []table.Row
	for user, days := range parsedSchedule {
		data = append(data, table.Row{user, shift, days["weekends"], days["holidays"], days["oncall"]})
	}

	sort.Slice(data, func(i, j int) bool { return data[i][4].(uint) > data[j][4].(uint) })

	fields := table.Row{"ENGINEER", "SHIFT", "WEEKEND", "HOLIDAY", "TOTAL"}
	printTable(data, fields)

}

func shiftNow(shifts map[string]string, token string) {
	var data []table.Row
	var mutex = &sync.Mutex{}
	var wg sync.WaitGroup
	for shift, shiftID := range shifts {
		wg.Add(1)
		go func(shift, shiftID string) {
			defer wg.Done()
			schedule, err := getSchedule(shiftID, "", "", token)
			if err != nil {
				log.Printf("Failed to get schedule for %s: %v", shift, err)
				return
			}

			var name string
			if schedule.PagerdutySchedule.Oncall != nil {
				name = schedule.PagerdutySchedule.Oncall.EntryUser.Name
			}
			mutex.Lock()
			data = append(data, table.Row{shift, name})
			mutex.Unlock()
		}(shift, shiftID)
	}
	wg.Wait()

	sort.Slice(data, func(i, j int) bool { return data[i][0].(string) < data[j][0].(string) })

	fields := table.Row{"SHIFT", "ENGINEER"}
	printTable(data, fields)
}

func shiftsRoster(shifts map[string]string, startdate, enddate, token string) {
	var data []table.Row
	rawData := make(map[string]map[string]string)
	var mutex = &sync.Mutex{}
	var wg sync.WaitGroup

	for shift, shiftID := range shifts {
		wg.Add(1)
		go func(shift, shiftID, startdate, enddate, token string) {
			defer wg.Done()
			schedule, err := getSchedule(shiftID, startdate, enddate, token)
			if err != nil {
				log.Printf("Failed to get schedule for %s: %v", shift, err)
				return
			}

			for _, entry := range schedule.PagerdutySchedule.FinalSchedule.RenderedScheduleEntries {
				start, err := convertTime(entry.Start, "")
				if err != nil {
					start = entry.Start
				}
				day, _ := weekday(entry.Start)

				mutex.Lock()
				if rawData[start] == nil {
					rawData[start] = map[string]string{}
				}

				rawData[start]["day"] = day.String()
				rawData[start][shift] = entry.EntryUser.Name
				mutex.Unlock()

			}
		}(shift, shiftID, startdate, enddate, token)
	}
	wg.Wait()

	for k, v := range rawData {
		data = append(
			data, table.Row{k, v["day"]})
	}

	sort.Slice(data, func(i, j int) bool { return data[i][0].(string) < data[j][0].(string) })

	fields := table.Row{"START", "DAY"}
	printTable(data, fields)
}

func shiftsUser(shifts map[string]string, name, startdate, enddate, token string) {
	var data []table.Row
	var userName string
	var mutex = &sync.Mutex{}
	var wg sync.WaitGroup

	name = strings.ToLower(name)
	for shift, shiftID := range shifts {
		wg.Add(1)
		go func(shift, shiftID, startdate, enddate, token string) {
			defer wg.Done()
			schedule, err := getSchedule(shiftID, startdate, enddate, token)
			if err != nil {
				log.Printf("Failed to get schedule for %s: %v", shift, err)
				return
			}

			var userID string
			for _, u := range schedule.PagerdutySchedule.Users {
				if strings.Contains(strings.ToLower(u.Name), name) {
					userID = u.ID
					userName = u.Name
					break
				}
			}

			if userID == "" {
				return
			}

			for _, entry := range schedule.PagerdutySchedule.FinalSchedule.RenderedScheduleEntries {
				if entry.EntryUser.ID == userID {

					start, err := convertTime(entry.Start, "")
					if err != nil {
						start = entry.Start
					}
					day, _ := weekday(entry.Start)

					mutex.Lock()
					data = append(data, table.Row{start, day.String(), shift})
					mutex.Unlock()
				}
			}

		}(shift, shiftID, startdate, enddate, token)
	}
	wg.Wait()

	sort.Slice(data, func(i, j int) bool { return data[i][0].(string) < data[j][0].(string) })

	fields := table.Row{"Start", "Day", "Shift"}
	fmt.Println("Schedule for", userName)
	printTable(data, fields)
}

func sprintPoints(shifts map[string]string, startdate, enddate, token string) {
	ids := map[string]interface{}{}
	sprintDays := workDays(startdate, enddate)

	parsedSchedule := make(map[string]uint)
	for shift, shiftID := range shifts {
		schedule, err := getSchedule(shiftID, startdate, enddate, token)
		if err != nil {
			log.Printf("Failed to get schedule for %s: %v", shift, err)
			continue
		}

		for _, entry := range schedule.PagerdutySchedule.FinalSchedule.RenderedScheduleEntries {
			if _, ok := ids[entry.EntryUser.ID]; !ok {
				continue
			}
			parsedSchedule[entry.EntryUser.Name]++
		}
	}

	var data []table.Row
	for user, onCallDays := range parsedSchedule {
		offShiftDays := sprintDays - onCallDays
		suggestedSP := storyPoints(offShiftDays)

		data = append(
			data, table.Row{
				user,
				fmt.Sprint(onCallDays),
				fmt.Sprint(offShiftDays),
				fmt.Sprintf("%.1f", float32(onCallDays)*100/float32(sprintDays)),
				fmt.Sprint(suggestedSP),
			})
	}

	sort.Slice(data, func(i, j int) bool { return data[i][0].(string) < data[j][0].(string) })

	fmt.Printf(" # of business days: %d\n", sprintDays)
	fields := table.Row{"engineer", "oncall days", "off shift days", "tactical %", "suggested SP"}
	printTable(data, fields)
}

func OpsRoster(shifts map[string]string, startdate, enddate, token string) {
	ids := map[string]interface{}{}
	var data []table.Row
	rawData := make(map[string]map[string]string)
	users := make(map[string]map[string]int)
	var totalBAU, totalOPS, total float32

	for shift, shiftID := range shifts {
		schedule, err := getSchedule(shiftID, startdate, enddate, token)
		if err != nil {
			log.Printf("Failed to get schedule for %s: %v", shift, err)
			continue
		}

		for _, entry := range schedule.PagerdutySchedule.FinalSchedule.RenderedScheduleEntries {
			if _, ok := ids[entry.EntryUser.ID]; !ok {
				continue
			}

			if users[entry.EntryUser.Name] == nil {
				users[entry.EntryUser.Name] = make(map[string]int)
			}
			users[entry.EntryUser.Name][shift]++
			total++
			if shift == "OPS" {
				totalOPS++
			} else {
				totalBAU++
			}

			start, err := convertTime(entry.Start, "2006-01-02")
			if err != nil {
				start = entry.Start
			}
			day, _ := weekday(entry.Start)

			if rawData[start] == nil {
				rawData[start] = map[string]string{}
			}

			rawData[start]["day"] = day.String()
			if _, ok := rawData[start][entry.EntryUser.Name]; ok {
				rawData[start][entry.EntryUser.Name] = fmt.Sprintf("%s, %s", rawData[start][entry.EntryUser.Name], shift)
			} else {
				rawData[start][entry.EntryUser.Name] = shift
			}
		}
	}

	usersKeys := make([]string, 0, len(users))
	for u := range users {
		usersKeys = append(usersKeys, u)
	}

	sort.Slice(usersKeys, func(i, j int) bool { return usersKeys[i] < usersKeys[j] })

	for k, v := range rawData {
		r := table.Row{k, v["day"]}
		for _, u := range usersKeys {
			r = append(r, v[u])
		}

		data = append(data, r)
	}

	sort.Slice(data, func(i, j int) bool { return data[i][0].(string) < data[j][0].(string) })

	fields := table.Row{"DATE", "DAY"}
	for _, u := range usersKeys {
		fields = append(fields, u)
	}

	printTable(data, fields)

	fields2 := table.Row{
		"user",
		"# ops", "% ops",
		"# bau", "% bau",
		"# tactical", "% tactical",
	}
	var data2 []table.Row
	for _, u := range usersKeys {
		pOPS := float32(users[u]["OPS"]) / totalOPS * 100
		pBAU := float32(users[u]["BAU"]) / totalBAU * 100
		uTotal := users[u]["OPS"] + users[u]["BAU"]
		pTotal := float32(uTotal) / total * 100
		data2 = append(
			data2,
			table.Row{
				u,
				users[u]["OPS"], fmt.Sprintf("%.2v", pOPS),
				users[u]["BAU"], fmt.Sprintf("%.2v", pBAU),
				uTotal, fmt.Sprintf("%.2v", pTotal),
			})
	}
	printTable(data2, fields2)
}
