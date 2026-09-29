module github.com/logpulse/logpulse/services/all

go 1.22.0

require (
	github.com/go-chi/chi/v5 v5.2.1
	github.com/go-chi/cors v1.2.1
	github.com/logpulse/logpulse/pkg/alertengine v0.0.0
	github.com/logpulse/logpulse/pkg/alerterstream v0.0.0
	github.com/logpulse/logpulse/pkg/auth v0.0.0
	github.com/logpulse/logpulse/pkg/ingestapi v0.0.0
	github.com/logpulse/logpulse/pkg/livehub v0.0.0
	github.com/logpulse/logpulse/pkg/notify v0.0.0
	github.com/logpulse/logpulse/pkg/redisx v0.0.0
	github.com/logpulse/logpulse/pkg/storage v0.0.0
	github.com/logpulse/logpulse/pkg/workerstream v0.0.0
)

require (
	github.com/ClickHouse/ch-go v0.63.1 // indirect
	github.com/ClickHouse/clickhouse-go/v2 v2.30.0 // indirect
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-faster/city v1.0.1 // indirect
	github.com/go-faster/errors v0.7.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.7.2 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/paulmach/orb v0.11.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/redis/go-redis/v9 v9.7.3 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	go.opentelemetry.io/otel v1.32.0 // indirect
	go.opentelemetry.io/otel/trace v1.32.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/logpulse/logpulse/pkg/alertengine => ../../pkg/alertengine
	github.com/logpulse/logpulse/pkg/alerterstream => ../../pkg/alerterstream
	github.com/logpulse/logpulse/pkg/auth => ../../pkg/auth
	github.com/logpulse/logpulse/pkg/ingestapi => ../../pkg/ingestapi
	github.com/logpulse/logpulse/pkg/livehub => ../../pkg/livehub
	github.com/logpulse/logpulse/pkg/notify => ../../pkg/notify
	github.com/logpulse/logpulse/pkg/redisx => ../../pkg/redisx
	github.com/logpulse/logpulse/pkg/storage => ../../pkg/storage
	github.com/logpulse/logpulse/pkg/workerstream => ../../pkg/workerstream
)
