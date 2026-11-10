module tools.xdoubleu.com/factory-runner

go 1.27.3

require (
	github.com/stretchr/testify v1.12.1
	tools.xdoubleu.com/selfupdate v0.0.0
)

require go.yaml.in/yaml/v3 v3.0.5 // indirect

replace tools.xdoubleu.com/selfupdate => ../selfupdate
