module grol.io/grol

go 1.22.5

require (
	fortio.org/cli v1.7.0
	fortio.org/log v1.16.0
	fortio.org/testscript v0.3.1 // only for tests
	fortio.org/version v1.0.4
	github.com/google/go-cmp v0.6.0 // only for tests
)

// replace fortio.org/log => ../../fortio.org/log

require (
	fortio.org/struct2env v0.4.1 // indirect
	github.com/kortschak/goroutine v1.1.2 // indirect
	golang.org/x/crypto/x509roots/fallback v0.0.0-20240626151235-a6a393ffd658 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/tools v0.8.0 // indirect
)
