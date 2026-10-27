FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/external-monitor .

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S monitor \
    && adduser -S -G monitor -h /nonexistent monitor
COPY --from=build /out/external-monitor /usr/local/bin/external-monitor
USER monitor
ENV CONFIG_FILE=/config/config.json \
    STATE_FILE=/data/state.json \
    LISTEN_ADDR=:8080 \
    TZ=America/Toronto
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/external-monitor"]
