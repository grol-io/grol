Following along https://interpreterbook.com and making changes/simplification/cleanups

Install/run it:
```shell
CGO_ENABLED=0 -trimpath -ldflags="-w -s" -tags no_net,no_json github.com/ldemailly/gorepl@latest
```

Dev mode:
```shell
go install golang.org/x/tools/cmd/stringer@latest
make # for stripped down executable including build tags etc to make it minimal
```

Status: done up to and including 3.8 - ie functional expressions, if etc with constants but no variable.

### Reading notes

- See the commit history for improvements/changes (e.g redundant state in lexer etc)

- [x] interface nil check in parser

- [x] Do we really need all these `let `, wouldn't `x = a + 3` be enough? made optional

- [ ] Seems like ast and object are redundant to a large extent

- [x] Introduced errors sooner, it's sort of obviously needed

- [x] Put handling of return/error once at the top instead of peppered all over

- [x] Make all the Eval functions receiver methods on State instead of passing environment around

- [x] made built ins like len() tokens (cheaper than carrying the string version during eval)

- [ ] fix up == and != in 3 places (int, string and default)

- [ ] change int to ... float? number? or add float or big int?

- [ ] use + for concat of arrays and merging of maps

- [ ] call maps maps and not hash (or maybe assoc array but that's long)
