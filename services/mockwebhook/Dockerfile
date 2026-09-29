FROM golang:1.22-alpine AS build
WORKDIR /src
COPY services/mockwebhook/main.go .
RUN CGO_ENABLED=0 go build -o /mockwebhook main.go

FROM alpine:3.20
RUN apk add --no-cache wget
COPY --from=build /mockwebhook /usr/local/bin/mockwebhook
EXPOSE 9090
ENTRYPOINT ["/usr/local/bin/mockwebhook"]
