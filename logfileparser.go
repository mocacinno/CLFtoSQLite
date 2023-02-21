package main

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/ini.v1"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

type args struct {
	filename          string
	padname           string
	dbname            string
	timeformat        string
	ignoredips        []string
	ignoredhostagents []string
	ignoredreferrers  []string
	ignoredrequests   []string
	mydomain          string
}

func parseargs() args {
	padPtr := flag.String("path", `./`, "the path to the log files")
	filesPtr := flag.String("files", `.*\.log.*\.gz`, "REGEX of the files to include")
	dbnamePtr := flag.String("dbname", `apachelog.db`, "name of the database to use")
	configfilePtr := flag.String("config", `none`, "complete path to the config file")
	mydomainPtr := flag.String("mydomain", `localhost.local`, "your domain name, so it doesn't show up as refferer")
	timeformatPtr := flag.String("timeformat", `02/Jan/2006:15:04:05 +0100`, "the timeformat to use for parsing")
	ignorehostagents := flag.String("ignore_hostagents", `.*google.*`, "ignore this hostagent. If you want to ignore multipe hostagents: use a configfile!")
	ignoredreferrers := flag.String("ignore_referrers", `.*localhost.*`, "ignore this referrer. If you want to ignore multipe referrers: use a configfile!")
	ignorevisitorips := flag.String("ignore_visitor_ips", `^127\.0\.0\1$`, "ignore this ip. If you want to ignore multipe ips: use a configfile!")
	ignoredrequests := flag.String("ignore_requests", `robots\.txt$`, "ignore this request. If you want to ignore multipe requests: use a configfile!")

	helpwithRegexPtr := flag.Bool("helpwithregex", false, "show regex examples and exit")
	flag.Parse()
	var ignorevisitorips_list []string
	var ignorehostagents_list []string
	var ignoredreferrers_list []string
	var ignoredrequests_list []string
	flag_configfile := *configfilePtr
	helpwithRegex := *helpwithRegexPtr
	if helpwithRegex {
		output := `
.*\.log.*\.gz => any character (.*) followed by .log (\.log) followed by any character (.*) followed by .gz (\.gz)
.*\.log\.\\d$ => any character (.*) followed by .log (\.log) followed by a dot (\.) followed by a digit (\\d) and nothing more ($)
`
		fmt.Printf("%s", output)
		os.Exit(0)
	}

	var output args
	if flag_configfile == `none` {
		if _, err := os.Stat("config.ini"); err == nil {
			fmt.Printf("config file was not entered, but i found a config.ini file in the current path... using that one\n")
			flag_configfile = "config.ini"
		}
		if _, err := os.Stat("/etc/CLFtoSQLite/config.ini"); err == nil {
			fmt.Printf("config file was not entered, but i found a config.ini file: /etc/CLFtoSQLite/config.ini... using that one\n")
			flag_configfile = "config.ini"
		}
	}
	if flag_configfile != `none` {
		cfg, err := ini.Load(flag_configfile)
		if err != nil {
			fmt.Printf("Fail to read file: %v", err)
			os.Exit(1)
		}
		output.filename = cfg.Section("parser").Key("fileregex").String()
		output.padname = cfg.Section("parser").Key("pad").String()
		output.dbname = cfg.Section("general").Key("dbfilepath").String()
		output.timeformat = cfg.Section("general").Key("timeformat").String()
		output.mydomain = cfg.Section("general").Key("mydomain").String()
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

	} else {
		output.filename = *filesPtr
		output.padname = *padPtr
		output.dbname = *dbnamePtr
		output.timeformat = *timeformatPtr
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
	var querylist []string
	querylist = append(querylist, "CREATE TABLE IF NOT EXISTS `user` (`id`    INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,`ip`    TEXT NOT NULL,`useragent`     TEXT);")
	querylist = append(querylist, "CREATE TABLE IF NOT EXISTS `request` (`id`    INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,`request`       TEXT NOT NULL);")
	querylist = append(querylist, "CREATE TABLE IF NOT EXISTS `referrer` (`id`    INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,`referrer`      TEXT NOT NULL);")
	querylist = append(querylist, "CREATE TABLE IF NOT EXISTS `alreadyloaded` (`id`    INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,`hash`      TEXT NOT NULL);")
	querylist = append(querylist, "CREATE TABLE IF NOT EXISTS `visit` ( `id` INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,`referrer` INTEGER NOT NULL, `request` INTEGER NOT NULL, `visit_timestamp` INTEGER NOT NULL, `user`  INTEGER NOT NULL, `statuscode` INTEGER, `httpsize` INTEGER, FOREIGN KEY(`request`) REFERENCES `request`(`id`),  FOREIGN KEY(`referrer`) REFERENCES `referrer`(`id`),  FOREIGN KEY(`user`) REFERENCES `user`(`id`) 	);")
	querylist = append(querylist, "CREATE INDEX IF NOT EXISTS user_ip_agent on user(ip,useragent);")
	querylist = append(querylist, "CREATE INDEX IF NOT EXISTS request_request on request(request);")
	querylist = append(querylist, "CREATE INDEX IF NOT EXISTS referrer_referrer on referrer(referrer);")
	for _, query := range querylist {
		_, err := db.Exec(query)
		if err != nil {
			fmt.Printf("%s\n", err.Error())
			os.Exit(1)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	return tx
}

func getfiles(regex string, pathS string, prepdb map[string]*sql.Stmt) []string {
	var files []string
	filepath.Walk(pathS, func(path string, f os.FileInfo, _ error) error {
		if !f.IsDir() {
			r, err := regexp.MatchString(regex, f.Name())
			if err == nil && r {
				filehandle, err := os.Open(pathS + f.Name())
				if err != nil {
					log.Fatal(err)
				}
				defer filehandle.Close()

				hash := sha256.New()
				if _, err := io.Copy(hash, filehandle); err != nil {
					log.Fatal(err)
				}
				filehash := hex.EncodeToString(hash.Sum(nil))
				stmt_countalreadyloaded := prepdb["stmt_countalreadyloaded"]
				var countalreadyloaded int
				stmt_countalreadyloaded.QueryRow(filehash).Scan(&countalreadyloaded)
				if countalreadyloaded == 0 {
					files = append(files, f.Name())
					fmt.Printf("%s added to the todo list\n", f.Name())
				} else {
					fmt.Printf("%s was already parsed in the past... skipping\n", f.Name())
				}

			}
		}
		return nil
	})
	return files
}

func parseme(line string, prepdb map[string]*sql.Stmt, maxvisittimestamp int, timeformat string, args args) bool {
	re := regexp.MustCompile(`(?m)^(\S*).*\[(.*)\]\s"(\S*)\s(\S*)\s([^"]*)"\s(\S*)\s(\S*)\s"([^"]*)"\s"([^"]*)"$`)
	match := re.FindStringSubmatch(line)
	if len(match) == 10 {
		ip := match[1]
		datumtijd := match[2]
		method := match[3]
		request := match[4]
		httpversion := match[5]
		returncode := match[6]
		httpsize := match[7]
		referrer := match[8]
		useragent := match[9]
		ignore := false
		for _, ignoredhostagent := range args.ignoredhostagents {
			r, err := regexp.MatchString(ignoredhostagent, useragent)
			if err == nil && r {
				ignore = true
			}
		}
		for _, ignoredip := range args.ignoredips {
			r, err := regexp.MatchString(ignoredip, ip)
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
		if ignore == false {
			insertrow(prepdb, ip, datumtijd, method, request, httpversion, returncode, httpsize, referrer, useragent, maxvisittimestamp, timeformat)
		}

	} else {
		fmt.Printf("unable to parse line %s %s", len(match), line)
	}
	return true
}

func prepstatements(tx *sql.Tx) map[string]*sql.Stmt {
	listitems := make(map[string]*sql.Stmt)
	/*
		alreadyloaded hash
	*/
	query_insertalreadyloaded := "insert into alreadyloaded(hash) values (?)"
	stmt_insertalreadyloaded, err := tx.Prepare(query_insertalreadyloaded)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	listitems["stmt_insertalreadyloaded"] = stmt_insertalreadyloaded

	query_countalreadyloaded := "select count(*) from alreadyloaded where hash = ?"
	stmt_countalreadyloaded, err := tx.Prepare(query_countalreadyloaded)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	listitems["stmt_countalreadyloaded"] = stmt_countalreadyloaded
	/*
		user table related statements
	*/

	query_insertuser := "insert into user(ip, useragent) values (?,?)"
	stmt_insertuser, err := tx.Prepare(query_insertuser)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	listitems["stmt_insertuser"] = stmt_insertuser

	query_countusers := "select count(*) from user where ip = ? and useragent = ?"
	stmt_countusers, err := tx.Prepare(query_countusers)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	listitems["stmt_countusers"] = stmt_countusers

	query_selectuserid := "select id from user where ip = ? and useragent = ?"
	stmt_selectuserid, err := tx.Prepare(query_selectuserid)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	listitems["stmt_selectuserid"] = stmt_selectuserid

	/*
		request table related statements
	*/
	query_insertrequest := "insert into request(request) values (?)"
	stmt_insertrequest, err := tx.Prepare(query_insertrequest)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	listitems["stmt_insertrequest"] = stmt_insertrequest

	query_countrequest := "select count(*) from request where request = ?"
	stmt_countrequest, err := tx.Prepare(query_countrequest)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	listitems["stmt_countrequest"] = stmt_countrequest

	query_selectrequestid := "select id from request where request = ?"
	stmt_selectrequestid, err := tx.Prepare(query_selectrequestid)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	listitems["stmt_selectrequestid"] = stmt_selectrequestid

	/*
		referrer table related statements
	*/
	query_insertreferrer := "insert into referrer(referrer) values (?)"
	stmt_insertreferrer, err := tx.Prepare(query_insertreferrer)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	listitems["stmt_insertreferrer"] = stmt_insertreferrer

	query_countreferrer := "select count(*) from referrer where referrer = ?"
	stmt_countreferrer, err := tx.Prepare(query_countreferrer)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	listitems["stmt_countreferrer"] = stmt_countreferrer

	query_selectreferrerid := "select id from referrer where referrer = ?"
	stmt_selectreferrerid, err := tx.Prepare(query_selectreferrerid)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	listitems["stmt_selectreferrerid"] = stmt_selectreferrerid

	/*
		visit table related statements
	*/

	query_insertvisit := "insert into visit(referrer, request,  visit_timestamp, user, statuscode, httpsize) values (?,?,?,?,?,?)"
	stmt_insertvisit, err := tx.Prepare(query_insertvisit)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	listitems["stmt_insertvisit"] = stmt_insertvisit

	query_maxvisittimestamp := "select max(visit_timestamp) from visit"
	stmt_maxvisittimestamp, err := tx.Prepare(query_maxvisittimestamp)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	listitems["stmt_maxvisittimestamp"] = stmt_maxvisittimestamp

	/*
		return
	*/
	return listitems
}

func insertrow(prepdb map[string]*sql.Stmt, ip string, datumtijd string, method string, request string, httpversion string, returncode string, httpsize string, referrer string, useragent string, maxtimestamp int, longForm string) {
	/*
		create user and return userid or return userid of existing user (userid)
	*/
	thetime, e := time.Parse(longForm, datumtijd)
	if e != nil {
		fmt.Printf("Can't parse time format")
	}
	epoch := thetime.Unix()
	/*
		visit_hour, visit_minute, visit_second := thetime.Clock()
		visit_year := thetime.Year()
		visit_month := thetime.Month()
		visit_day := thetime.Day()
	*/
	//fmt.Printf("\nDEBUG: ik heb timestamp %s en verwacht formaat %s. Ik haal hieruit %d/%d/%d %d:%d:%d en maakte hiervan de unix timestamp %d\n", datumtijd, longForm, visit_day, visit_month, visit_year, visit_hour, visit_minute, visit_second, epoch)

	if int(epoch) > maxtimestamp {
		stmt_countusers := prepdb["stmt_countusers"]
		var numberofusers int
		stmt_countusers.QueryRow(ip, useragent).Scan(&numberofusers)

		var userid int
		if numberofusers > 0 {
			//user already exists... get his id :)
			stmt_selectuserid := prepdb["stmt_selectuserid"]
			stmt_selectuserid.QueryRow(ip, useragent).Scan(&userid)
			//fmt.Printf("user already exists with userid %d\n", userid)
		} else {
			//user does not exist... create the bugger
			stmt_insertuser := prepdb["stmt_insertuser"]
			stmt_insertuser_result, err := stmt_insertuser.Exec(ip, useragent)
			if err != nil {
				fmt.Printf("%s\n", err.Error())
				os.Exit(1)
			}
			var id64 int64
			id64, err = stmt_insertuser_result.LastInsertId()
			userid = int(id64)
			//fmt.Printf("created a new user and assigned id %d\n", userid)
		}

		/*
			create request and return requestid or return requestid of existing request (requestid)
		*/
		stmt_countrequest := prepdb["stmt_countrequest"]
		var numberofrequests int
		stmt_countrequest.QueryRow(request).Scan(&numberofrequests)
		var requestid int
		if numberofrequests > 0 {
			stmt_selectrequestid := prepdb["stmt_selectrequestid"]
			stmt_selectrequestid.QueryRow(request).Scan(&requestid)
			//fmt.Printf("request already exists with requestid %d\n", requestid)
		} else {
			stmt_insertrequest := prepdb["stmt_insertrequest"]

			stmt_insertrequest_result, err := stmt_insertrequest.Exec(request)
			if err != nil {
				fmt.Printf("%s\n", err.Error())
				os.Exit(1)
			}
			var id64 int64
			id64, err = stmt_insertrequest_result.LastInsertId()
			requestid = int(id64)
			//fmt.Printf("created a new request and assigned id %d\n", requestid)
		}

		/*
			create referrer and return referrerid or return referrerid of existing referrer (referrerid)
		*/
		stmt_countreferrer := prepdb["stmt_countreferrer"]
		var numberofreferrers int
		stmt_countreferrer.QueryRow(referrer).Scan(&numberofreferrers)
		var referrerid int
		if numberofreferrers > 0 {
			stmt_selectreferrerid := prepdb["stmt_selectreferrerid"]
			stmt_selectreferrerid.QueryRow(referrer).Scan(&referrerid)
			//fmt.Printf("referrer already exists with referrerid %d\n", referrerid)
		} else {
			stmt_insertreferrer := prepdb["stmt_insertreferrer"]
			stmt_insertreferrer_result, err := stmt_insertreferrer.Exec(referrer)
			if err != nil {
				fmt.Printf("%s\n", err.Error())
				os.Exit(1)
			}
			var id64 int64
			id64, err = stmt_insertreferrer_result.LastInsertId()
			referrerid = int(id64)
			//fmt.Printf("created a new refferer and assigned id %d\n", referrerid)
		}
		/*
			get max timestamp of current db and insert newer records
		*/
		stmt_insertvisit := prepdb["stmt_insertvisit"]
		stmt_insertvisit.Exec(referrerid, requestid, int(epoch), userid, returncode, httpsize)
	}

}

func getmaxvisittimestamp(prepdb map[string]*sql.Stmt) int {
	stmt_maxvisittimestamp := prepdb["stmt_maxvisittimestamp"]
	var output int
	stmt_maxvisittimestamp.QueryRow().Scan(&output)
	return output
}

func InsertParsedFileHashIntoDb(filename string, filepath string, prepdb map[string]*sql.Stmt) {

	filehandle, err := os.Open(filepath + filename)
	if err != nil {
		log.Fatal(err)
	}
	defer filehandle.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, filehandle); err != nil {
		log.Fatal(err)
	}
	filehash := hex.EncodeToString(hash.Sum(nil))
	/*
		query_insertalreadyloaded := prepdb["query_insertalreadyloaded"]
		query_insertalreadyloaded.Exec(filehash)
	*/
	//fmt.Printf("%s", filehash)
	stmt_insertalreadyloaded := prepdb["stmt_insertalreadyloaded"]

	_, err = stmt_insertalreadyloaded.Exec(filehash)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}

}

func main() {
	arguments := parseargs()
	db := createdb(arguments.dbname)
	defer db.Close()
	tx := initialisedb(db)
	prepdb := prepstatements(tx)
	maxvisittimestamp := getmaxvisittimestamp(prepdb)
	var scanner *bufio.Scanner

	fmt.Printf("we're going to parse all files that look like %s in path %s\n", arguments.filename, arguments.padname)
	files := getfiles(arguments.filename, arguments.padname, prepdb)
	for _, filename := range files {
		fmt.Printf("starting with %s\n", filename)
		file, err := os.Open(arguments.padname + filename)
		defer file.Close()
		if err != nil {
			log.Fatal(err)
		}
		r, err := regexp.MatchString(`.*\.gz`, filename)
		if err == nil && r {
			gz, err := gzip.NewReader(file)
			if err != nil {
				log.Fatal(err)
			}
			defer gz.Close()
			scanner = bufio.NewScanner(gz)
		} else {
			scanner = bufio.NewScanner(file)
		}
		for scanner.Scan() {
			currentline := scanner.Text()
			parseme(currentline, prepdb, maxvisittimestamp, arguments.timeformat, arguments)
		}

		InsertParsedFileHashIntoDb(filename, arguments.padname, prepdb)

	}
	tx.Commit()

}
