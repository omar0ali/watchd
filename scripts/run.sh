#!/bin/bash

ENV=$1

#
# LOADING THE APP FROM INSIDE T_TOOL FOLDER
# ONLY FOR TESTING. THE APP MUST BE INSTALLED IN BIN
# FOLDER TO BE WORKING PROPERLY.
#

if [ "$ENV" = "dev" ]; then
    go run ../main.go "${@:2}" ## THIS IS FIXED PATH
elif [ "$ENV" = "build" ]; then
    echo "Building binary..."
    go build -o build/watchdaemon ./cmd/watchd-daemon
    go build -o build/watchd .
    echo "./build/watchd"
    echo "./build/watchddaemon"
else
    echo "Usage: .scripts/run.sh [dev|test|build]"
    exit 1
fi
