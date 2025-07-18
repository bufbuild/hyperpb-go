module buf.build/go/hyperpb/test

go 1.24

replace buf.build/go/hyperpb => ../

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.6-20250625184727-c923a0c2a132.1
	buf.build/go/hyperpb v0.0.0-00010101000000-000000000000
	github.com/planetscale/vtprotobuf v0.6.0
	github.com/protocolbuffers/protoscope v0.0.0-20221109213918-8e7a6aafa2c9
	github.com/stretchr/testify v1.10.0
	google.golang.org/protobuf v1.36.6
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/timandy/routine v1.1.5 // indirect
)
