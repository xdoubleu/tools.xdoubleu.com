module tools.xdoubleu.com/gateway

go 1.26.0

require (
	github.com/getsentry/sentry-go v0.46.2
	github.com/joho/godotenv v1.5.1
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	tools.xdoubleu.com/sentrytools v0.0.0
)

replace tools.xdoubleu.com/sentrytools => ../sentrytools
