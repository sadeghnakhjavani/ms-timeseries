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

# ClickHouse's official entrypoint starts a temporary server for initdb.d, then
# SIGTERM's it and exec's the real server. Wait until schema exists and the
# server stays reachable across that restart.
wait_for_stable_clickhouse() {
	local consecutive=0
	local i
	for i in $(seq 1 180); do
		if clickhouse-client --query "EXISTS TABLE default.symbols" 2>/dev/null | grep -qx 1 \
			&& clickhouse-client --query "SELECT 1" >/dev/null 2>&1; then
			consecutive=$((consecutive + 1))
			if [[ "${consecutive}" -ge 5 ]]; then
				return 0
			fi
		else
			consecutive=0
		fi
		sleep 1
	done
	echo "ClickHouse failed to become ready within 180 seconds" >&2
	return 1
}

ensure_clickhouse_schema() {
	local exists
	exists="$(clickhouse-client --query "EXISTS TABLE default.jalali_calendar" 2>/dev/null || echo 0)"
	if [[ "${exists}" == "1" ]]; then
		return 0
	fi

	echo "ClickHouse schema missing; applying /app/clickhouse/init/*.sql"
	local f
	for f in /app/clickhouse/init/*.sql; do
		[[ -f "${f}" ]] || continue
		echo "  applying $(basename "${f}")"
		clickhouse-client --multiquery <"${f}"
	done
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

wait_for_stable_clickhouse
ensure_clickhouse_schema
seed_jalali_calendar_if_needed

echo "Starting ms-timeseries API..."
# Keep ClickHouse running; do not run stop_clickhouse on successful exec path.
trap - EXIT
exec /app/ms-timeseries
