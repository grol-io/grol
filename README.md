<a id="grol"></a>
<img alt="GROL" src="https://grol.io/grol_mascot.png" width="33%"/>

## History

Initially created by following along https://interpreterbook.com and making many changes/simplification/cleanups. And pretty much a complete rewrite in 0.25 with interning, everything a node/expression (flatter ast), etc... (kept the tests though).

There is also now a [discord bot](https://github.com/grol-io/grol-discord-bot#grol-discord-bot) as well as a `wasm` version that runs directly in your browser, try it on [grol.io](https://grol.io/)

## Install

Install/run it:
```shell
CGO_ENABLED=0 go install -trimpath -ldflags="-w -s" -tags no_net,no_json grol.io/grol@latest
```

Or with docker:
```
docker run -ti ghcr.io/grol-io/grol:latest
```

On a mac
```
brew install grol-io/tap/grol
```

Or get one of the [binary releases](https://github.com/grol-io/grol/releases)

## What it does
Sample:
```go
$ grol -parse
10:53:12.675 grol 0.29.0 go1.22.5 arm64 darwin - welcome!
$ fact = func(n) {if (n<=1) {return 1} n*self(n-1)} // could be n*fact(n-1) too
== Parse ==> fact = func(n) {
	if n <= 1 {
		return 1
	}
	n * self(n - 1)
} // could be n*fact(n-1) too
== Eval  ==> func(n){if n<=1{return 1}n*self(n-1)}
$ n=fact(6)
== Parse ==> n = fact(6)
== Eval  ==> 720
$ m=fact(7)
== Parse ==> m = fact(7)
== Eval  ==> 5040
$ m/n
== Parse ==> m / n
== Eval  ==> 7
$ func fx(n) {if n>0 {return fx(n-1)}; info.all_ids}; fx(3)
== Parse ==> func fx(n) {
	if n > 0 {
		return fx(n - 1)
	}
	info.all_ids
}
fx(3)
== Eval  ==> {0:["E","PI","abs","fact","fx","log2","n","printf"],1:["n","self"],2:["fx","n","self"],3:["fx","n","self"],4:["fx","n","self"]}
$ info["gofuncs"] // other way to access map keys, for when they aren't strings for instance
== Parse ==> info["gofuncs"] // other way to access map keys, for when they aren't strings for instance
== Eval  ==> ["acos","asin","atan","ceil","cos","eval","exp","floor","json","ln","log10","pow","round","sin","sprintf","sqrt","tan","trunc","unjson"]
$ info.keywords
== Parse ==> info.keywords
== Eval  ==> ["else","error","false","first","func","if","len","log","macro","print","println","quote","rest","return","true","unquote"]
```

## Language features

Functional int, float, string and boolean expressions

Functions, lambdas, closures (including recursion in anonymous functions, using `self()`)

Arrays, maps (including map.key as map["key"] shorthand access)

print, log

macros and more all the time (like canonical reformat using `grol -format` and wasm/online version etc)

automatic memoization

easy extensions/adding Go functions to grol (see [extensions/extension.go](extensions/extension.go) for a lot of `math` additions)

variadic functions both Go side and grol side (using `..` on grol side)

Use `info` to see all the available functions, keywords, operators etc... (can be used inside functions too to examine the stack)

See also [sample.gr](examples/sample.gr) and others in that folder, that you can run with
```
gorepl examples/*.gr
```

or copypaste to the online version on [grol.io](https://grol.io)

## Dev mode:
```shell
go install golang.org/x/tools/cmd/stringer@latest
make # for stripped down executable including build tags etc to make it minimal
```

### Reading notes

See [Open Issues](https://grol.io/grol/issues) for what's left to do

- See the commit history for improvements/changes (e.g redundant state in lexer etc)

- [x] interface nil check in parser

- [x] Do we really need all these `let `, wouldn't `x = a + 3` be enough? made optional

- Seems like ast and object are redundant to a large extent

- [x] Introduced errors sooner, it's sort of obviously needed

- [x] Put handling of return/error once at the top instead of peppered all over

- [x] Make all the Eval functions receiver methods on State instead of passing environment around

- [x] made built ins like len() tokens (cheaper than carrying the string version during eval)

- fix up == and != in 3 places (int, string and default)

- change int to ... float? number? or rather add float/double (maybe also or big int?...)

- [x] use + for concat of arrays and merging of maps

- [x] call maps maps and not hash (or maybe assoc array but that's long)

- [x] don't make a slice to join with , when there is already a strings builder. replace byte buffers by string builder.

- [x] generalized tokenized built in (token id based instead of string)

-  Add "extension" internal functions (calling into a go function), with variadic params, param types etc

- [x] Identifiers are letter followed by alphanum*

- [x] map of interface correctly equals the actual underlying types, no need for custom hashing
  -> implies death to pointers (need non pointer receiver and use plain objects and not references)

- unicode (work as is in strings already)

- [x] flags for showing parse or not (default not pass `-parse` to see parsing)

- [x] file input vs stdin repl (made up .gr for gorepl)

- actual name for the language - it's not monkey (though it's monkey ~~compatible~~ derived, just better/simpler/...)

- multiline support in stdin repl

- [x] add >= and <= comparison operators

- [x] add comments support (line)
   - add /* */ style

- line numbers for errors (for file mode)

- [x] use `func` instead of `fn` for functions

- [x] figure out how to get syntax highlighting (go style closest - done thx to viulisti -> .gitattributes)

- assignment to maps keys and arrays indexes

- for loop

- [x] switched to non pointer receivers in Object and (base/integer) Ast so equality checks in maps work without special hashing (big win)

### CLI Usage

```
grol 0.38.0 usage:
	grol [flags] *.gr files to interpret or `-` for stdin without prompt
  or no arguments for stdin repl...
or 1 of the special arguments
	grol {help|envhelp|version|buildinfo}
flags:
  -c string
    	command/inline script to run instead of interactive mode
  -compact
    	When printing code, use no indentation and most compact form
  -eval
    	show eval results (default true)
  -format
    	don't execute, just parse and re format the input
  -history string
    	history file to use (default "~/.grol_history")
  -parse
    	show parse tree
  -shared-state
    	All files share same interpreter state (default is new state for each)
```
(excluding logger control, see `gorepl help` for all the flags, of note `-logger-no-color` will turn off colors for gorepl too, for development there are also `-profile*` options for pprof, when building without `no_pprof`)
