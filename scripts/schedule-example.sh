#!/bin/sh
set -eu

curl --fail-with-body --silent --show-error \
  --request POST "${SERVICE_URL:-http://localhost:8080}/follow-ups" \
  --header 'Content-Type: application/json' \
  --data '{
    "work_order": {
      "id": "WO-1042",
      "photo_urls": ["https://photos.example/WO-1042/completed.jpg"],
      "dispatch_status": "dispatched"
    },
    "delay_hours": 6
  }'
