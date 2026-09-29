module github.com/logpulse/logpulse/pkg/alertengine

go 1.22

require (
	github.com/logpulse/logpulse/pkg/logevent v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/logpulse/logpulse/pkg/logevent => ../logevent
