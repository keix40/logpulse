FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.work go.work.sum ./
COPY pkg ./pkg
COPY services ./services
WORKDIR /src/services/alerter
RUN go mod download && CGO_ENABLED=0 go build -o /alerter .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /alerter /usr/local/bin/alerter
COPY deploy/alerts.yaml /etc/logpulse/alerts.yaml
EXPOSE 8082
ENTRYPOINT ["/usr/local/bin/alerter"]
