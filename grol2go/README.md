# Grol2go

Currently generates a (go) binary embedding grol script(s)

Example with fib.gr

```go
func fib(x) {
	if x <= 1 {
		x
	} else {
		fib(x - 1) + fib(x - 2)
	}
}
println(fib(90))
```

```sh
16:20:39 grol2go grol2go$ go run . -dest ./gotmp fib.gr
16:20:43.753 [INF] Compiling 1 grol file to Go in "./gotmp" using module name "fib"
go: creating new go.mod: module fib
16:20:43.762 [INF] Running 'go mod tidy' in "./gotmp"
go: finding module for package grol.io/grol/repl
go: finding module for package grol.io/grol/eval
go: finding module for package grol.io/grol/extensions
go: found grol.io/grol/eval in grol.io/grol v0.96.0
go: found grol.io/grol/extensions in grol.io/grol v0.96.0
go: found grol.io/grol/repl in grol.io/grol v0.96.0
16:20:44.197 [INF] Running 'go build ./gotmp'
16:20:46.361 [INF] Code embedding completed successfully. Run with:
./gotmp/fib
```

And if you run it:

```sh
$ ./gotmp/fib
2880067194370816120
```
