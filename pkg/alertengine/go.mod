module github.com/logpulse/logpulse/pkg/alertengine

go 1.22

require (
	github.com/logpulse/logpulse/pkg/logevent v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/kr/pretty v0.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
)

replace github.com/logpulse/logpulse/pkg/logevent => ../logevent
