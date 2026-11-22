module tools.xdoubleu.com/factory-runner

go 1.27.0

require (
	github.com/stretchr/testify v1.12.1
	tools.xdoubleu.com/selfupdate v0.0.0
	tools.xdoubleu.com/wardcase v0.0.0
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/coder/websocket v1.8.15
	github.com/xdoubleu/diff-coverage v0.1.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/oauth2 v0.38.0 // indirect
)

replace tools.xdoubleu.com/selfupdate => ../selfupdate

replace tools.xdoubleu.com/wardcase => ../wardcase

tool github.com/xdoubleu/diff-coverage
