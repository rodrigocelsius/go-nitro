# Validation (registry + plugin BR)

**English:** [README.md](README.md) · **Português:** [README.pt.md](README.pt.md)

Shows:

- `nitro.NewValidator` / **per-app** validator
- Brazilian documents via `nitro.RegisterBRValidators` (`usecases/validator/br` plugin, not builtins)
- Alias `same=Password` (≡ `eqfield`)
- Custom messages (`msg_cpf`, `msg_same`)
- `nitro.Parse[User](c)` → automatic 400/422

## Run

```bash
go run .
```

## Smoke

```bash
# 422 — required fields
curl -s -X POST localhost:8080/users -H 'Content-Type: application/json' -d '{}'

# 422 — invalid CPF + mismatched passwords
curl -s -X POST localhost:8080/users -H 'Content-Type: application/json' \
  -d '{"cpf":"000","password":"a","confirm":"b"}'

# 201 — OK
curl -s -X POST localhost:8080/users -H 'Content-Type: application/json' \
  -d '{"cpf":"529.982.247-25","password":"secret","confirm":"secret"}'
```

## Layout

```text
validation/
  main.go   # NewValidator + RegisterBRValidators + Parse
```

**App** rules (SKU, …) stay in the app (`v.Register`). Docs: [EN](../../../docs/guide/validation.md) · [PT](../../../docs/pt/guide/validation.md).
