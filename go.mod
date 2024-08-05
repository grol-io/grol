module grol.io/grol

go 1.22.5

require (
	fortio.org/cli v1.8.0
	fortio.org/log v1.16.0
	fortio.org/testscript v0.3.1 // only for tests
	fortio.org/version v1.0.4
	github.com/google/go-cmp v0.6.0 // only for tests
)

// replace fortio.org/log => ../../fortio.org/log

require fortio.org/sets v1.1.1

require (
	fortio.org/struct2env v0.4.1 // indirect
	github.com/kortschak/goroutine v1.1.2 // indirect
	golang.org/x/crypto/x509roots/fallback v0.0.0-20240626151235-a6a393ffd658 // indirect
	golang.org/x/exp v0.0.0-20240604190554-fc45aab8b7f8 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/tools v0.22.0 // indirect
)
