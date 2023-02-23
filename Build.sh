#!/bin/bash
#go tool dist list
sed "s/vmyversion/v$1/" nfpm.yaml.template > nfpm.yaml
rm builds/*
GOOS=linux go build -ldflags="-s -w" -o builds/logfileparser.tmp logfileparser.go
GOOS=linux go build -ldflags="-s -w" -o builds/stats.tmp stats.go
upx -f --brute -o builds/logfileparser builds/logfileparser.tmp
upx -f --brute -o builds/stats builds/stats.tmp
cp config.template.ini builds/config.ini
tar cvzf builds/clftosqlite-v$1-linux-amd64.tar.gz builds/logfileparser builds/stats builds/config.ini
GOOS=windows GOARCH=amd64 go build -o builds/logfileparser.exe logfileparser.go
GOOS=windows GOARCH=amd64 go build -o builds/stats.exe stats.go
7zr a builds/clftosqlite-v$1-windows-amd64.zip builds/*.exe builds/config.ini
nfpm pkg --packager deb --target /root/CLFtoSQLite/builds/
nfpm pkg --packager rpm --target /root/CLFtoSQLite/builds/
gpg --detach-sign builds/*.deb
gpg --detach-sign builds/*.rpm
gpg --detach-sign builds/clftosqlite-v$1-linux-amd64.tar.gz
gpg --detach-sign builds/clftosqlite-v$1-windows-amd64.zip
rm builds/stats*
rm builds/logfileparser*
rm builds/*.ini