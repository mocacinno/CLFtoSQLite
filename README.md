# CLFtoSQLite

combined logfile from apache parsing, putting all data in sqlite and visualising said data

## About The Project

I had an apache webserver running... All default... Combined log format... And i wanted some stats.  
In the open source world, i ended up with 3 choices:  

* awstats 
* webalizer
* goaccess

all other tools seem to be worthless or paying. And those 3 were either allmost unmaintained, or not really what i wanted.  
So i tought to myself: why not add one more :smile:

### Built With

golang


## Getting Started

It's a go program, vendoring included... just clone it, verify the sourcecode, build and run... Or use my precompiled binary's.

### Prerequisites

* go
* linux (eventough it should also be compileable on windows)
* apache combined logs

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
   ./logparser
   ```

1. run the stats

   ```sh
   ./stats
   ```

## Roadmap

* [ ] stats uses a cursor to read data. I need to use a struct so i can re-use
* [ ] I don't want to load all entries... I need to filter some entries out
* [ ] stats needs graphs

## License

Distributed under the Apache2.0 License.