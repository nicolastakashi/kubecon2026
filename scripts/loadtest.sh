#!/bin/bash

# Load generator for Packet Sorter
# Generates continuous traffic to demonstrate metrics

BASE_URL="${BASE_URL:-http://localhost:8080}"

echo "Packet Sorter Load Generator"
echo "Base URL: $BASE_URL"
echo "Press Ctrl+C to stop"
echo ""

# Array of possible outcomes
OUTCOMES=("ok" "rework" "drop")
REASONS=("damaged" "wrong_address" "weight_issue" "inspection")

generate_traffic() {
    # Ingest a new parcel
    RESPONSE=$(curl -s -X POST "$BASE_URL/api/parcels")
    PARCEL_ID=$(echo $RESPONSE | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    
    if [ -z "$PARCEL_ID" ]; then
        echo "Failed to ingest parcel"
        return
    fi
    
    echo "Created: $PARCEL_ID"
    
    # Random delay between ingest and process (100ms - 500ms)
    sleep $(echo "scale=3; 0.1 + $RANDOM/32767*0.4" | bc -l)
    
    # Process the parcel with random outcome
    OUTCOME=${OUTCOMES[$RANDOM % ${#OUTCOMES[@]}]}
    
    if [ "$OUTCOME" == "rework" ]; then
        REASON=${REASONS[$RANDOM % ${#REASONS[@]}]}
        curl -s -X POST "$BASE_URL/api/parcels/$PARCEL_ID/process" \
            -H "Content-Type: application/json" \
            -d "{\"outcome\": \"$OUTCOME\", \"reason\": \"$REASON\"}" > /dev/null
        echo "  → Processed: $OUTCOME ($REASON)"
    else
        curl -s -X POST "$BASE_URL/api/parcels/$PARCEL_ID/process" \
            -H "Content-Type: application/json" \
            -d "{\"outcome\": \"$OUTCOME\"}" > /dev/null
        echo "  → Processed: $OUTCOME"
    fi
    
    # If outcome was ok, ship it
    if [ "$OUTCOME" == "ok" ]; then
        sleep $(echo "scale=3; 0.1 + $RANDOM/32767*0.2" | bc -l)
        curl -s -X POST "$BASE_URL/api/parcels/$PARCEL_ID/ship" > /dev/null
        echo "  → Shipped!"
    fi
}

# Main loop
while true; do
    generate_traffic
    # Random delay between parcels (200ms - 1s)
    sleep $(echo "scale=3; 0.2 + $RANDOM/32767*0.8" | bc -l)
done