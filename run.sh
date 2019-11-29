#!/usr/bin/env bash
case $1 in
"windows")
	export CGO_ENABLED=1
	export GOARCH=amd64
	export GOOS=windows
	export GO111MODULE=on
	export CC=x86_64-w64-mingw32-gcc
	export CXX=x86_64-w64-mingw32-g++
	go build -v -mod=vendor -ldflags "-X 'github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo.Version=victoria-metrics-20191115-104921-heads-master-0-gc56b9ed-dirty-5f10b01e'" -ldflags '-w -s -extldflags "-static"' -o bin/victoria-metrics-win.exe github.com/VictoriaMetrics/VictoriaMetrics/app/victoria-metrics
	;;
"linux")
	go build -v -mod=vendor -o bin/victoria-metrics-linux github.com/VictoriaMetrics/VictoriaMetrics/app/victoria-metrics
	;;
esac
