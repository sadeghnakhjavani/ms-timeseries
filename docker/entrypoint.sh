#!/usr/bin/env bash
set -euo pipefail

CLICKHOUSE_PID=""

stop_clickhouse() {
	if [[ -n "${CLICKHOUSE_PID}" ]] && kill -0 "${CLICKHOUSE_PID}" 2>/dev/null; then
		kill "${CLICKHOUSE_PID}" 2>/dev/null || true
		wait "${CLICKHOUSE_PID}" 2>/dev/null || true
	fi
}

trap stop_clickhouse EXIT INT TERM

wait_for_clickhouse() {
	for _ in $(seq 1 60); do
		if clickhouse-client --query "SELECT 1" >/dev/null 2>&1; then
			return 0
		fi
		sleep 1
	done
	echo "ClickHouse failed to become ready within 60 seconds" >&2
	return 1
}

seed_jalali_calendar_if_needed() {
	local count
	count="$(clickhouse-client --query "SELECT count() FROM jalali_calendar" 2>/dev/null || echo 0)"
	if [[ "${count}" == "0" ]]; then
		echo "jalali_calendar is empty; running /app/jalali-seed"
		/app/jalali-seed
	fi
}

echo "Starting ClickHouse..."
/entrypoint.sh &
CLICKHOUSE_PID=$!

wait_for_clickhouse
seed_jalali_calendar_if_needed

echo "Starting marketick API..."
exec /app/marketick
