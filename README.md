# go-import-vehicles

## Introduction

The goal of this app is to collect shared mobility data of different providers into one database.

This database can be used by municipalities and other govermental organisations that are granted access by [CROW](https://crow.nl/).

The data can be accessed by going to https://deelfietsdashboard.nl/ and https://dashboarddeelmobiliteit.nl/ .

## Features

This app runs continuously:

- It polls MDS/GBFS/TOMP API's
- It stores aggregated data in the postgresql database

## How to install

Install go, see https://go.dev/doc/install

Install redis, i.e. `sudo apt-get install redis`

Install tile38, i.e. https://github.com/tidwall/tile38/releases

Run:

    export DEV=false
    export DB_NAME=deelfietsdashboard
    export DB_USER=deelfietsdashboard
    export DB_HOST=localhost
    export DB_PASSWORD=X
    export REDIS_HOST=localhost:6379
    export TILE38_HOST=localhost:9851

Run:
    
    go run .

## Questions?

Email to info@deelfietsdashboard.nl
