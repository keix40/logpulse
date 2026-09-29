module github.com/logpulse/logpulse/services/ingest

go 1.22.0

toolchain go1.22.2

require (
	github.com/alicebob/miniredis/v2 v2.34.0
	github.com/go-chi/chi/v5 v5.2.1
	github.com/logpulse/logpulse/pkg/auth v0.0.0
	github.com/logpulse/logpulse/pkg/ingestapi v0.0.0
	github.com/logpulse/logpulse/pkg/logevent v0.0.0
	github.com/logpulse/logpulse/pkg/redisx v0.0.0
	github.com/redis/go-redis/v9 v9.7.3
)

require (
	github.com/alicebob/gopher-json v0.0.0-20230218143504-906a9b012302 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
)

replace (
	github.com/logpulse/logpulse/pkg/auth => ../../pkg/auth
	github.com/logpulse/logpulse/pkg/ingestapi => ../../pkg/ingestapi
	github.com/logpulse/logpulse/pkg/logevent => ../../pkg/logevent
	github.com/logpulse/logpulse/pkg/redisx => ../../pkg/redisx
)
