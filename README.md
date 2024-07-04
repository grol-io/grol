Following along https://interpreterbook.com and making changes/simplification/cleanups

Required (dev or even run as I don't like checking in generated files, though I guess I will eventually):
```
go install golang.org/x/tools/cmd/stringer@latest
go generate ./...
# or
make build # for stripped down executable including build tags etc to make it minimal
```

Status: done up to and including 3.8 - ie functional expressions, if etc with constants but no variable.

### Reading notes

- See the commit history for improvements/changes (e.g redundant state in lexer etc)

- interface nil check in parser

- Do we really need all these `let `, wouldn't `x = a + 3` be enough?

- Seems like ast and object are redundant to a large extent

- Introduced errors sooner, it's sort of obviously needed

- Put handling of return/error once at the top instead of peppered all over

- Make all the Eval functions receiver methods on State instead of passing environment around
