<a id="grol"></a>
<img alt="GROL" src="https://grol.io/grol_mascot.png" width="33%"/>

## History

Initially created by following along https://interpreterbook.com and making many changes/simplification/cleanups. And pretty much a complete rewrite in 0.25 with interning, everything a node/expression (flatter ast), etc... (kept the tests though).

There is also now a [discord bot](https://github.com/grol-io/grol-discord-bot#grol-discord-bot) as well as a `wasm` version that runs directly in your browser, try it on [grol.io](https://grol.io/)

## Install

Install/run it:
```shell
CGO_ENABLED=0 go install -trimpath -ldflags="-w -s" -tags "no_net,no_json,no_pprof" grol.io/grol@latest
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
$ func fx(n,s) {if n>0 {return fx(n-1,s)}; info.stack}; fx(3,"abc")
== Parse ==> func fx(n, s) {
	if n > 0 {
		return fx(n - 1, s)
	}
	info.stack
}
fx(3, "abc")
== Eval  ==> [{"s":true},{"s":true},{"s":true},{"s":true}] // registers don't show up, so only s does and not n	
$ info["gofuncs"] // other way to access map keys, for when they aren't strings for instance
== Parse ==> info["gofuncs"] // other way to access map keys, for when they aren't strings for instance
== Eval  ==> ["acos","asin","atan","ceil","cos","eval","exp","floor","json","ln","log10","pow","round","sin","sprintf","sqrt","tan","trunc","unjson"]
$ info.keywords
== Parse ==> info.keywords
== Eval  ==> ["else","error","false","first","func","if","len","log","macro","print","println","quote","rest","return","true","unquote"]
```

The interactive repl mode has extra features:
- Editable history (use arrow keys, Ctrl-A etc...) to navigate previous commands
- Hit the `<tab>` key at any time to get id/keywords/function completion
- `history` command to see the current history, prefixed by a number
- You can use for instance `!23` to repeat the 23rd statement
- State is auto saved/loaded from `.gr` file in current directory unless `-no-auto` is passed
- A short `help`

## Language features

Functional int, float, string and boolean expressions

Functions, lambdas, closures (including recursion in anonymous functions, using `self()`)

Arrays, ordered maps (including map.key as map["key"] shorthand access and ability to put any type, including arrays, maps and functions as keys)

print, log

macros and more all the time (like canonical reformat using `grol -format` and wasm/online version etc)

automatic memoization

for loops (in addition to recursion based iterations)

easy extensions/adding Go functions to grol (see [extensions/extension.go](extensions/extension.go) for a lot of `math` additions)

variadic functions both Go side and grol side (using `..` on grol side)

Use `info` to see all the available functions, keywords, operators etc... (can be used inside functions too to examine the stack)

`save("filename")` and `load("filename")` current state.

See also [sample.gr](examples/sample.gr) and others in that folder, that you can run with
```shell
GOMEMLIMIT=1GiB grol examples/*.gr
```

or copypaste to the online version on [grol.io](https://grol.io).

There is also more involved code in [grol-io/grol-discord-bot/discord.gr](https://github.com/grol-io/grol-discord-bot/blob/main/discord.gr).

## Dev mode:
```shell
go install golang.org/x/tools/cmd/stringer@latest
make # for stripped down executable including build tags etc to make it minimal
```

### Reading notes
See [Open Issues](https://grol.io/grol/issues) for what's left to do

<details><summary>Click for detailed reading notes</summary>

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
</details>

### CLI Usage

```
grol 0.72.0 usage:
	grol [flags] *.gr files to interpret or `-` for stdin without prompt or no arguments for stdin repl...
or 1 of the special arguments
	grol {help|envhelp|version|buildinfo}
flags:
  -c string
    	command/inline script to run instead of interactive mode
  -compact
    	When printing code, use no indentation and most compact form
  -empty-only
    	only allow load()/save() to ./.gr
  -eval
    	show eval results (default true)
  -format
    	don't execute, just parse and reformat the input
  -history file
    	history file to use (default "~/.grol_history")
  -max-depth int
    	Maximum interpreter depth (default 149999)
  -max-duration duration
    	Maximum duration for a script to run. 0 for unlimited.
  -max-history size
    	max history size, use 0 to disable. (default 99)
  -max-save-len int
    	Maximum len of saved identifiers, use 0 for unlimited (default 4000)
  -no-auto
    	don't auto load/save the state to ./.gr
  -no-load-save
    	disable load/save of history
  -panic
    	Don't catch panic - only for development/debugging
  -parse
    	show parse tree
  -parse-debug
    	show all parenthesis in parse tree (default is to simplify using precedence)
  -quiet
    	Quiet mode, sets loglevel to Error (quietly) to reduces the output
  -restrict-io
    	restrict IOs (safe mode)
  -s	#! script mode: next argument is a script file to run, rest are args to the script
  -shared-state
    	All files share same interpreter state (default is new state for each)
```
(excluding logger control, see `gorepl help` for all the flags, of note `-logger-no-color` will turn off colors for gorepl too, for development there are also `-profile*` options for pprof, when building without `no_pprof`)

If you don't want to pass a flag and want to permanently change the `grol` history file location from your HOME directory, set `GROL_HISTORY_FILE` in the environment.
