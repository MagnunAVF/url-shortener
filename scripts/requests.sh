#!/bin/bash

MAX_REQUESTS=${1:-100}
BASE_URL=${BASE_URL:-http://localhost:8080}
SHORT_CODE=${2:-ZjrLu5SJWb}
TARGET_URL="${BASE_URL}/${SHORT_CODE}"

for i in $(seq 1 "$MAX_REQUESTS"); do
    echo "Doing request $i"
    curl "$TARGET_URL"
done
