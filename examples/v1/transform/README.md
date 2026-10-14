# Transform tags (`transform:`)

Demonstrates the bind pipeline: **default → transform → validate**.

```bash
go test .
# or: go run .  then POST /contacts
```

```json
{"name":"  joão da silva ","phone":"(11) 98877-6655","uf":"sp"}
```

→ `João da Silva` / `11988776655` / `SP`
