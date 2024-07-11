#!/usr/bin/env bash

# requires jq and irfetch (https://github.com/popmonkey/irdata/releases)
KEYFILE=$1
CREDSFILE=$2
LEAGUE_NAME=$3
LEAGUE_ID=$4

set -ex

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

JQ=$(which jq)
IRFETCH="${SCRIPT_DIR}/irfetch -c"

roster_json=$(mktemp /tmp/roster-json.XXXXXX)
roster_csv="/tmp/$LEAGUE_NAME-roster.csv"

$IRFETCH $KEYFILE $CREDSFILE "data/league/roster?league_id=$LEAGUE_ID" > $roster_json

jq -r -L. 'include "json2csv"; .roster | json2csv' $roster_json > $roster_csv

rm $roster_json
