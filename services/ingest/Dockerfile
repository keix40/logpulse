FROM golang:1.22-alpine AS build
WORKDIR /src
COPY pkg ./pkg
COPY services/ingest ./services/ingest
WORKDIR /src/services/ingest
ENV GOWORK=off
RUN go mod download && CGO_ENABLED=0 go build -o /ingest .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /ingest /usr/local/bin/ingest
EXPOSE 8080 5514
ENTRYPOINT ["/usr/local/bin/ingest"]
