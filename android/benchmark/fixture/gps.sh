#!/usr/bin/env bash
# Drives the emulator along route 7 of bench-gtfs.zip, one fix a second, so Tracking has live adherence while the
# benchmarks run. Stop it with Ctrl-C. ANDROID_SERIAL picks the emulator when more than one is running.
set -euo pipefail

serial=${ANDROID_SERIAL:-emulator-5554}
i=0
while true; do
    lat=$(awk -v i="$i" 'BEGIN { printf "%.6f", 47.5802 + (i % 120) * 0.0003 }')
    adb -s "$serial" emu geo fix -122.3658 "$lat" >/dev/null
    i=$((i + 1))
    sleep 1
done
