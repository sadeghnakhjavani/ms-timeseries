FROM golang:1.23-bookworm AS builder

ENV GOPROXY=https://goproxy.io,direct
ENV GOSUMDB=off

WORKDIR /build

COPY app/go.mod app/go.sum ./app/
RUN cd app && go mod download

COPY app/ ./app/
RUN cd app && CGO_ENABLED=0 GOOS=linux go build -o /marketick .

COPY scripts/go.mod scripts/go.sum ./scripts/
RUN cd scripts && go mod download

COPY scripts/ ./scripts/
RUN cd scripts && CGO_ENABLED=0 GOOS=linux go build -o /jalali-seed seed_jalali_calendar.go
RUN cd scripts && CGO_ENABLED=0 GOOS=linux go build -o /sample-seed seed_sample_data.go
RUN cd scripts && CGO_ENABLED=0 GOOS=linux go build -o /drop-database drop_database.go

FROM clickhouse:26.3.25.2-jammy

COPY clickhouse/init/ /docker-entrypoint-initdb.d/
COPY clickhouse/init/ /app/clickhouse/init/
COPY clickhouse/config/config.xml /etc/clickhouse-server/config.d/marketick-config.xml
COPY clickhouse/config/users.d/default-network.xml /etc/clickhouse-server/users.d/default-network.xml

COPY --from=builder /marketick /app/marketick
COPY --from=builder /jalali-seed /app/jalali-seed
COPY --from=builder /sample-seed /app/sample-seed
COPY --from=builder /drop-database /app/drop-database
COPY scripts/ /app/scripts/

COPY docker/entrypoint.sh /entrypoint-marketick.sh
RUN chmod +x /entrypoint-marketick.sh

ENV CH_HOST=localhost \
	CH_PORT=9000 \
	CH_USER=default \
	CH_PASSWORD= \
	CH_DATABASE=default \
	APP_PORT=8080

EXPOSE 8080 8123 9000

ENTRYPOINT ["/entrypoint-marketick.sh"]
