package main

import (
	"database/sql"
	"flag"
	"fmt"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	_ "github.com/mattn/go-sqlite3"
	"github.com/wcharczuk/go-chart"
	"gopkg.in/ini.v1"
	"math/rand"
	"os"
	//_ "reflect"
	"html/template"
	"regexp"
	"strconv"
	"time"
	"runtime"
	//"log"
	//	"github.com/wcharczuk/go-chart/drawing"
)

type Table struct {
	Pagetitle       string
	Pagedescription string
	Headers         map[string]string
	Data            []map[string]string
}

type Visit struct {
	id         int
	referrer   string
	request    string
	timestamp  int
	statuscode int
	httpsize   int
}

type Visitor struct {
	visitor_id int
	ip         string
	useragent  string
	visit      []Visit
}

type timeseriesplot_html struct {
	Title string
	Img string
	Description string
}
const timeseriesplot_tmpl = `
<h1>{{.Title}}</h1>
<p>{{.Description}}</p>
<img src="{{.Img}}">
`

const table_tmpl = `<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<title>{{.Pagetitle}}</title>
		<!-- choose a theme file -->
		<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/jquery.tablesorter/2.31.3/css/theme.default.min.css">
		<!-- load jQuery and tablesorter scripts -->
		<script type="text/javascript" src="https://cdnjs.cloudflare.com/ajax/libs/jquery/3.6.3/jquery.min.js"></script>
		<script type="text/javascript" src="https://cdnjs.cloudflare.com/ajax/libs/jquery.tablesorter/2.31.3/js/jquery.tablesorter.js"></script>
		
		<!-- tablesorter widgets (optional) -->
		<script type="text/javascript" src="https://cdnjs.cloudflare.com/ajax/libs/jquery.tablesorter/2.31.3/js/jquery.tablesorter.widgets.js"></script>

	</head>
	<body>
		<h1>{{.Pagetitle}}</h1>
		<p>{{.Pagedescription}}</p>
		<p>

		<table  id="myTable" class="tablesorter" border = "1">
		<thead>

			<tr>
				{{range .Headers}}
				<th>{{.}}</th>
				{{end}}
			</tr>
			</thead>
			<tbody>
		{{range .Data}}
			<tr>
				{{range .}}
				<td>{{.}}</td>
				{{end}}
			</tr>
		{{end}}
		</tbody>
		</table>
		</p>
		<script>
		$(function() {
			$("#myTable").tablesorter();
		  });
		</script>
	</body>
</html>`

type args struct {
	outputpad                string
	dbpad                    string
	max_rows_in_table        int
	number_of_days_detailed  int
	number_of_days_per_hour  int
	number_of_days_per_day   int
	number_of_days_per_week  int
	number_of_days_per_month int
	ignoredips               []string
	ignoredhostagents        []string
	ignoredreferrers         []string
	ignoredrequests          []string
	mydomain                 string
}

func drawdraw() {
	graph := chart.Chart{
		Series: []chart.Series{
			chart.ContinuousSeries{
				XValues: []float64{1.0, 2.0, 3.0, 4.0},
				YValues: []float64{1.0, 2.0, 3.0, 4.0},
			},
		},
	}

	f, _ := os.Create("./output/output.png")
	defer f.Close()
	graph.Render(chart.PNG, f)
}

func createBarChart() {
	// create a new bar instance
	bar := charts.NewBar()

	// Set global options
	bar.SetGlobalOptions(charts.WithTitleOpts(opts.Title{
		Title:    "Bar chart in Go",
		Subtitle: "This is fun to use!",
	}))

	// Put data into instance
	bar.SetXAxis([]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"}).
		AddSeries("Category A", generateBarItems()).
		AddSeries("Category B", generateBarItems())
	f, _ := os.Create("./output/bar.html")
	_ = bar.Render(f)
}

func generateBarItems() []opts.BarData {
	items := make([]opts.BarData, 0)
	for i := 0; i < 6; i++ {
		items = append(items, opts.BarData{Value: rand.Intn(500)})
	}
	return items
}

func parseargs() args {
	var output args
	padPtr := flag.String("outputpath", `./output`, "the output path")
	dbnamePtr := flag.String("dbname", `apachelog.db`, "name of the database to use")
	mydomainPtr := flag.String("mydomain", `localhost.local`, "your domain name, so it doesn't show up as refferer")
	number_of_days_detailed := flag.Int("number_of_days_detailed", 31, "number of days you want to show detailed info about")
	number_of_days_per_hour := flag.Int("number_of_days_per_hour", 31, "number of days you want to show hourly statistics about")
	number_of_days_per_day := flag.Int("number_of_days_per_day", 31, "number of days you want to show dayly statistics about")
	number_of_days_per_week := flag.Int("number_of_days_per_week", 31, "number of days you want to show weekly statistics about")
	number_of_days_per_month := flag.Int("number_of_days_per_month", 31, "number of days you want to show monthly statistics about")
	max_rows_in_tablePtr := flag.Int("max_rows_in_table", 10000, "maximum number of rows in a html table")
	ignorehostagents := flag.String("ignore_hostagents", `.*google.*`, "ignore this hostagent. If you want to ignore multipe hostagents: use a configfile!")
	ignoredreferrers := flag.String("ignore_referrers", `.*localhost.*`, "ignore this referrer. If you want to ignore multipe referrers: use a configfile!")
	ignorevisitorips := flag.String("ignore_visitor_ips", `^127\.0\.0\1$`, "ignore this ip. If you want to ignore multipe ips: use a configfile!")
	ignoredrequests := flag.String("ignore_requests", `robots\.txt$`, "ignore this request. If you want to ignore multipe requests: use a configfile!")
	configfilePtr := flag.String("config", `none`, "complete path to the config file")
	flag.Parse()
	flag_configfile := *configfilePtr
	var ignorevisitorips_list []string
	var ignorehostagents_list []string
	var ignoredreferrers_list []string
	var ignoredrequests_list []string
	if flag_configfile != `none` {
		cfg, err := ini.Load(flag_configfile)
		if err != nil {
			fmt.Printf("Fail to read file: %v", err)
			os.Exit(1)
		}
		output.outputpad = cfg.Section("output").Key("pad").String()
		output.number_of_days_detailed, _ = cfg.Section("output").Key("number_of_days_detailed").Int()
		output.max_rows_in_table, _ = cfg.Section("output").Key("max_rows_in_table").Int()
		output.dbpad = cfg.Section("general").Key("dbfilepath").String()
		output.mydomain = cfg.Section("general").Key("mydomain").String()
		output.number_of_days_per_hour, _ = cfg.Section("output").Key("number_of_days_per_hour").Int()
		output.number_of_days_per_day, _ = cfg.Section("output").Key("number_of_days_per_day").Int()
		output.number_of_days_per_week, _ = cfg.Section("output").Key("number_of_days_per_week").Int()
		output.number_of_days_per_month, _ = cfg.Section("output").Key("number_of_days_per_month").Int()
		for _, ignoredip := range cfg.Section("ignorevisitorips").Keys() {
			ignorevisitorips_list = append(ignorevisitorips_list, ignoredip.String())
		}
		output.ignoredips = ignorevisitorips_list

		for _, ignoredhostagent := range cfg.Section("ignorehostagents").Keys() {
			ignorehostagents_list = append(ignorehostagents_list, ignoredhostagent.String())
		}
		output.ignoredhostagents = ignorehostagents_list

		for _, ignoredreferrer := range cfg.Section("ignorereferrers").Keys() {
			ignoredreferrers_list = append(ignoredreferrers_list, ignoredreferrer.String())
		}
		output.ignoredreferrers = ignoredreferrers_list

		for _, ignoredrequest := range cfg.Section("ignoredrequests").Keys() {
			ignoredrequests_list = append(ignoredrequests_list, ignoredrequest.String())
		}
		output.ignoredrequests = ignoredrequests_list
		//ignoredreferrers
	} else {
		output.outputpad = *padPtr
		output.dbpad = *dbnamePtr
		output.max_rows_in_table = *max_rows_in_tablePtr
		output.number_of_days_detailed = *number_of_days_detailed
		output.number_of_days_per_hour = *number_of_days_per_hour
		output.number_of_days_per_day = *number_of_days_per_day
		output.number_of_days_per_week = *number_of_days_per_week
		output.number_of_days_per_month = *number_of_days_per_month
		output.mydomain = *mydomainPtr
		ignorevisitorips_list = append(ignorevisitorips_list, *ignorevisitorips)
		output.ignoredips = ignorevisitorips_list
		ignorehostagents_list = append(ignorehostagents_list, *ignorehostagents)
		output.ignoredhostagents = ignorehostagents_list
		ignoredreferrers_list = append(ignoredreferrers_list, *ignoredreferrers)
		output.ignoredreferrers = ignoredreferrers_list
		ignoredrequests_list = append(ignoredrequests_list, *ignoredrequests)
		output.ignoredrequests = ignoredrequests_list
	}
	return output
}

func createdb(dbnaam string) *sql.DB {
	db, err := sql.Open("sqlite3", dbnaam)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	return db
}

func initialisedb(db *sql.DB) *sql.Tx {
	tx, err := db.Begin()
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	return tx
}

func prepstatements(tx *sql.Tx, args args) map[string]*sql.Stmt {
	listitems := make(map[string]*sql.Stmt)
	/*
		detailed info about all visits
	*/
	query_allvisits_detailed := " select visit.id as visit_id, referrer.referrer as referrer, request.request as request,   visit.visit_timestamp as visit_timestamp, user.ip as user_ip, user.useragent as user_agent, visit.statuscode as visit_statuscode, visit.httpsize as visit_httpsize, user.id as user_id "
	query_allvisits_detailed += " from visit, user, request, referrer "
	query_allvisits_detailed += " where visit.referrer = referrer.id and visit.request = request.id and visit.user = user.id "
	query_allvisits_detailed += " and visit_timestamp > ? "
	query_allvisits_detailed += " order by visit_timestamp desc "
	//fmt.Printf("%s", query_allvisits_detailed)
	stmt_allvisits_detailed, err := tx.Prepare(query_allvisits_detailed)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	listitems["stmt_allvisits_detailed"] = stmt_allvisits_detailed

	query_nbhitsperday := "select count(*), date(visit_timestamp, 'unixepoch'), max(visit_timestamp) from visit group by date(visit_timestamp, 'unixepoch')"
	//fmt.Printf("%s", query_allvisits_detailed)
	stmt_nbhitsperday, err := tx.Prepare(query_nbhitsperday)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	listitems["stmt_nbhitsperday"] = stmt_nbhitsperday


	return listitems
}

func getdetailedstats_andfillstructs(args args, prepdb map[string]*sql.Stmt) (map[int]Visitor) {
	visitorlog := make(map[int]Visitor)

	nu := int(time.Now().Unix())
	vanaf := nu - (args.number_of_days_detailed * 86400)
	MyHeaders := map[string]string{
		"Title_0":  "nb",
		"Title_1":  "timestamp",
		"Title_1b": "request",
		"Title_2":  "referrer",
		"Title_3":  "user_ip",
		"Title_4":  "user_agent",
		"Title_5":  "visit_statuscode",
		"Title_6":  "visit_httpsize",
	}
	myTable := Table{
		Pagetitle:       "detailed visitor log",
		Pagedescription: "this page shows a detailed log of all visits over the last " + strconv.Itoa(args.number_of_days_detailed) + " days",
		Headers:         MyHeaders,
		Data:            []map[string]string{},
	}
	//fmt.Printf("timestamp moet groter zijn dan %d - (%d * 86400) = %d\n", nu, args.number_of_days_detailed, vanaf)
	stmt_allvisits_detailed := prepdb["stmt_allvisits_detailed"]
	rows, err := stmt_allvisits_detailed.Query(vanaf)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
	}
	defer rows.Close()
	rownum := 0
	for rows.Next() {
		rownum = rownum + 1
		var visit_id, visit_timestamp, visit_statuscode, visit_httpsize, user_id int
		var referrer, request, user_ip, user_agent string
		if err := rows.Scan(&visit_id, &referrer, &request, &visit_timestamp, &user_ip, &user_agent, &visit_statuscode, &visit_httpsize, &user_id); err != nil {
			fmt.Printf("%s\n", err.Error())
		}



		ignore := false
		for _, ignoredhostagent := range args.ignoredhostagents {
			r, err := regexp.MatchString(ignoredhostagent, user_agent)
			if err == nil && r {
				ignore = true
			}
		}
		for _, ignoredip := range args.ignoredips {
			r, err := regexp.MatchString(ignoredip, user_ip)
			if err == nil && r {
				ignore = true
			}
		}
		for _, ignoredreferrer := range args.ignoredreferrers {
			r, err := regexp.MatchString(ignoredreferrer, referrer)
			if err == nil && r {
				ignore = true
			}
		}
		for _, ignoredrequest := range args.ignoredrequests {
			r, err := regexp.MatchString(ignoredrequest, request)
			if err == nil && r {
				ignore = true
			}
		}
		if ignore == false  {
			//fmt.Printf("visit_id : %d, referrer: %s, request: %s,visit_day: %d, visit_month: %d, visit_year: %d,visit_hour : %d, visit_minute: %d, visit_second: %d,visit_timestamp: %d, user_ip: %s, user_agent: %s,visit_statuscode: %d,visit_httpsize%d\n\n", visit_id, referrer, request,visit_day, visit_month, visit_year,visit_hour, visit_minute, visit_second,visit_timestamp, user_ip, user_agent,visit_statuscode,visit_httpsize)
			visitstruct, exits := visitorlog[user_id]
			MyVisit := Visit{id: visit_id, referrer: referrer, request: request, timestamp: visit_timestamp, statuscode: visit_statuscode, httpsize: visit_httpsize }
			if exits {
				visitstruct.visit = append(visitstruct.visit, MyVisit)
				visitorlog[user_id] = visitstruct
			} else {
				MyVisitor := Visitor {visitor_id: user_id, ip: user_ip, useragent: user_agent}
				MyVisitor.visit = append(MyVisitor.visit)
				visitorlog[user_id] = MyVisitor
				// de visitstruct bestaat nog niet, ik moet hem aanmaken én de visit appenden
			}
			if (rownum <= args.max_rows_in_table) {
				MyData := map[string]string{
					"Value_0":  strconv.Itoa(rownum),
					"Value_1":  strconv.Itoa(visit_timestamp),
					"Value_1b": request,
					"Value_2":  referrer,
					"Value_3":  user_ip,
					"Value_4":  user_agent,
					"Value_5":  strconv.Itoa(visit_statuscode),
					"Value_6":  strconv.Itoa(visit_httpsize),
				}
				myTable.Data = append(myTable.Data, MyData)
				//fmt.Printf("%+v\n", myTable)
			}
			
		}

	}

	if err := rows.Err(); err != nil {
		fmt.Printf("%s\n", err.Error())
	}
	//fmt.Printf("%+v\n", myTable)
	//table_tmpl
	t, err := template.New("mytemplate").Parse(table_tmpl)
	if err != nil {
		panic(err)
	}
	var outputHTMLFile *os.File
	if outputHTMLFile, err = os.Create(args.outputpad + "detailed_visitor_log.html"); err != nil {
		panic(err)
	}

	if err = t.Execute(outputHTMLFile, myTable); err != nil {
		panic(err)
	}
	defer outputHTMLFile.Close()
	return visitorlog
}

func visitgraphs(map[int]Visitor, args) bool {
	return true
}

func overviewgraphs(args args, prepdb map[string]*sql.Stmt) bool {
	var XValues []time.Time
	var YValues []float64
	stmt_nbhitsperday := prepdb["stmt_nbhitsperday"]
	rows, err := stmt_nbhitsperday.Query()
	if err != nil {
		fmt.Printf("%s\n", err.Error())
	}
	defer rows.Close()
	rownum := 0
	for rows.Next() {
		rownum = rownum + 1
		var aantalhits int
		var datum string
		var avgepoch int
		if err := rows.Scan(&aantalhits, &datum, &avgepoch); err != nil {
			fmt.Printf("%s\n", err.Error())
		}
		golangtime := time.Unix(int64(avgepoch),0)
		XValues = append(XValues, golangtime)
		YValues = append(YValues, float64(aantalhits))
	}
	gochart_drawtimeseries(XValues, YValues, args, "Date", "Number of hits", "NbHitsPerDay.png", "NbHitsPerDay.html", "Number of hits per day", "The number of raw hits per day")
	return true
}

func gochart_drawtimeseries(XValues []time.Time, YValues []float64, args args, xtitle string, ytitle string, outputfilename_image string, outputfilename_html string, htmltitle string, description string) {
	graph := chart.Chart{
		Series: []chart.Series{
			chart.TimeSeries{
				XValues: XValues,
				YValues: YValues,
			},
		},
		XAxis: chart.XAxis{
            Name:      xtitle,
            NameStyle: chart.StyleShow(),
            Style:     chart.StyleShow(),
        },
        YAxis: chart.YAxis{
            Name:      ytitle,
            NameStyle: chart.StyleShow(),
            Style:     chart.StyleShow(),
        },
	}

	f, _ := os.Create(args.outputpad + outputfilename_image)
	defer f.Close()
	graph.Render(chart.PNG, f)


	myHtmlInput := timeseriesplot_html {
		Title: htmltitle,
		Img: outputfilename_image,
		Description: description,
	}
	t, err := template.New("mytemplate").Parse(timeseriesplot_tmpl)
	if err != nil {
		panic(err)
	}
	var outputHTMLFile *os.File
	if outputHTMLFile, err = os.Create(args.outputpad + outputfilename_html); err != nil {
		panic(err)
	}

	if err = t.Execute(outputHTMLFile, myHtmlInput); err != nil {
		panic(err)
	}
	defer outputHTMLFile.Close()


}

func main() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	//fmt.Printf("runtime memstats begin of proces %+v\n", memStats.Alloc)
	args := parseargs()
	//fmt.Printf("starting with parameters %+v\n", args)
	runtime.ReadMemStats(&memStats)
	//fmt.Printf("runtime memstats after argument parsing %+v\n", memStats.Alloc)
	db := createdb(args.dbpad)
	defer db.Close()
	tx := initialisedb(db)
	runtime.ReadMemStats(&memStats)
	//fmt.Printf("runtime memstats after initilialising db %+v\n", memStats.Alloc)
	prepdb := prepstatements(tx, args)
	runtime.ReadMemStats(&memStats)
	//fmt.Printf("runtime memstats after prep statements %+v\n", memStats.Alloc)
	visitors := getdetailedstats_andfillstructs(args, prepdb)
	visitgraphs(visitors, args)
	overviewgraphs(args,prepdb)
	tx.Commit()
	runtime.ReadMemStats(&memStats)
	//fmt.Printf("runtime memstats end of proces %+v\n", memStats.Alloc)
	//createBarChart()
	//drawdraw()
}
