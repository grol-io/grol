Following along https://interpreterbook.com and making changes/simplification/cleanups


Required (dev):
```
go install golang.org/x/tools/cmd/stringer@latest
go generate ./...
```

### Reading notes

- See the commit history for improvements/changes (e.g redundant state in lexer etc)

- interface nil check in parser

- Do we really need all these `let `, wouldn't `x = a + 3` be enough?
