module github.com/limpo1989/arena/cmd/protoc-gen-arena/testdata/wiretest

go 1.24.0

require (
	github.com/limpo1989/arena v0.0.0-00010101000000-000000000000
	github.com/limpo1989/arena/cmd/protoc-gen-arena/testdata/arenatest v0.0.0-00010101000000-000000000000
	github.com/limpo1989/arena/cmd/protoc-gen-arena/testdata/stdproto v0.0.0-00010101000000-000000000000
	google.golang.org/protobuf v1.36.11
)

replace (
	github.com/limpo1989/arena => ../../../..
	github.com/limpo1989/arena/cmd/protoc-gen-arena/testdata/arenatest => ../arenatest
	github.com/limpo1989/arena/cmd/protoc-gen-arena/testdata/stdproto => ../stdproto
)
