module github.com/logpulse/logpulse/pkg/ingestapi

go 1.22.0

require (
	github.com/logpulse/logpulse/pkg/logevent v0.0.0
	github.com/logpulse/logpulse/pkg/redisx v0.0.0
	github.com/redis/go-redis/v9 v9.7.3
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
)

replace (
	github.com/logpulse/logpulse/pkg/logevent => ../logevent
	github.com/logpulse/logpulse/pkg/redisx => ../redisx
)
