module github.com/logpulse/logpulse/pkg/notify

go 1.22.0

require github.com/logpulse/logpulse/pkg/alertengine v0.0.0

require (
	github.com/kr/text v0.2.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/logpulse/logpulse/pkg/alertengine => ../alertengine
