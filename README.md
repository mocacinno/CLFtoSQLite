# CLFtoSQLite

**important: this project is currently in pre-alpha... Some parts work, others don't!!! It's useless to open issues at this point since everything is a work in progress!!!**  
combined logfile from apache parsing, putting all data in sqlite and visualising said data

## About The Project

I had an apache webserver running... All default... Combined log format... And i wanted some stats.  
In the open source world, i ended up with 3 choices:  

* awstats 
* webalizer
* goaccess

all other tools seem to be worthless or paying. And those 3 were either allmost unmaintained, or not really what i wanted.  
So i tought to myself: why not add one more :smile:

## Visual schema

![visual schema of data flow](clftosqlite.jpg "visual schema of data flow")

### Built With

golang


## Getting Started

It's a go program, vendoring included... just clone it, verify the sourcecode, build and run... Or use my precompiled binary's.

### Prerequisites

* go
* linux (eventough it should also be compileable on windows)
* apache combined logs
* space... Usually your logfiles will be rotated and gzipped. I'll be unpacking them and putting them into a relational database (which produces some more overhead). For every ~70 Mb of UNCOMPRESSED logs, you'll need about 100Mb of diskspace
* time. Eventough compiled go runs pretty fast :smile:. The initial testload of about ~70 Mb of logfiles took about 40 seconds on my old XEON dedicated server. That's about 1 second per 2 Mb of logfiles. Subsequent loads will mostly skip already loaded files and it'll skip records that are older than the newest timestamp in the db of changed logfiles.

### Installation

1. Clone the repo

   ```sh
   git clone https://github.com/mocacinno/CLFtoSQLite.git
   ```

1. build

   ```sh
   go build logfileparser.go
   go build stats.go
   ```

1. copy (or directly edit) config.template.ini (you can swap out nano by vi or any other editor)

   ```sh
   cp config.template.ini config.ini
   nano config.ini
   ```

## Usage

1. build the sqlite database and fill it. You need to run this tool every time you want to load the ascii logs into the sqlite database for analysis/graphing

   ```sh
   ./logparser -config config.ini
   ```

1. run the stats

   ```sh
   ./stats -config config.ini
   ```

## Roadmap

* [x] stats uses a cursor to read data. I need to use a struct so i can re-use
* [x] I don't want to load all entries... I need to filter some entries out
* [x] stats needs graphs

## License

Distributed under the Apache2.0 License.
