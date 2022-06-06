module github.com/brholstein/docker-machine-driver-vz

go 1.17

require (
	github.com/Code-Hex/vz v0.0.5-0.20220605095544-71c01f183afe
	github.com/docker/machine v0.16.2
	github.com/mitchellh/go-ps v1.0.0
	github.com/pkg/errors v0.9.1
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/docker v20.10.13+incompatible // indirect
	github.com/google/go-cmp v0.5.5 // indirect
	github.com/hectane/go-acl v0.0.0-20190604041725-da78bae5fc95 // indirect
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6 // indirect
	github.com/rs/xid v1.4.0 // indirect
	github.com/stretchr/testify v1.7.1 // indirect
	golang.org/x/crypto v0.0.0-20220315160706-3147a52a75dd // indirect
	golang.org/x/sys v0.0.0-20220429233432-b5fbb4746d32 // indirect
	golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/docker/machine => github.com/machine-drivers/machine v0.7.1-0.20210719174735-6eca26732baa

replace github.com/Code-Hex/vz => github.com/brholstein/vz v0.0.5-0.20220605095544-71c01f183afe
