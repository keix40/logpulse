FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.work go.work.sum* ./
COPY pkg ./pkg
COPY services/worker ./services/worker
WORKDIR /src/services/worker
RUN go mod download && CGO_ENABLED=0 go build -o /worker .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /worker /usr/local/bin/worker
EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/worker"]
