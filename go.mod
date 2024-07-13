module github.com/ldemailly/gorepl

go 1.22.5

require (
	fortio.org/cli v1.7.0
	fortio.org/log v1.14.0
	github.com/google/go-cmp v0.6.0 // only for tests
)

require (
	fortio.org/struct2env v0.4.1 // indirect
	fortio.org/version v1.0.4 // indirect
	golang.org/x/crypto/x509roots/fallback v0.0.0-20240626151235-a6a393ffd658 // indirect; not actually used with our build tags
)
