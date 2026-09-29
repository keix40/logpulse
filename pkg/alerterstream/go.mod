module github.com/logpulse/logpulse/pkg/alerterstream

go 1.22.0

require (
	github.com/logpulse/logpulse/pkg/alertengine v0.0.0
	github.com/logpulse/logpulse/pkg/logevent v0.0.0
	github.com/logpulse/logpulse/pkg/notify v0.0.0
	github.com/logpulse/logpulse/pkg/redisx v0.0.0
	github.com/redis/go-redis/v9 v9.7.3
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/logpulse/logpulse/pkg/alertengine => ../alertengine
	github.com/logpulse/logpulse/pkg/logevent => ../logevent
	github.com/logpulse/logpulse/pkg/notify => ../notify
	github.com/logpulse/logpulse/pkg/redisx => ../redisx
)
