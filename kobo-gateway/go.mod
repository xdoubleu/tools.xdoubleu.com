module tools.xdoubleu.com/kobo-gateway

go 1.27.3

require (
	github.com/getsentry/sentry-go v0.50.0
	github.com/stretchr/testify v1.12.1
	tools.xdoubleu.com/selfupdate v0.0.0
)

require (
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

replace tools.xdoubleu.com/selfupdate => ../selfupdate
