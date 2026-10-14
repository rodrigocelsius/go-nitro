# Validation (registry + plugin BR)

**English:** [README.md](README.md) · **Português:** [README.pt.md](README.pt.md)

Demonstra:

- `nitro.NewValidator` / validator **por App**
- Documentos BR via `nitro.RegisterBRValidators` (plugin `usecases/validator/br`, não builtins)
- Alias `same=Password` (≡ `eqfield`)
- Mensagens custom (`msg_cpf`, `msg_same`)
- `nitro.Parse[User](c)` → 400/422 automáticos

## Run

```bash
go run .
```

## Smoke

```bash
# 422 — campos obrigatórios
curl -s -X POST localhost:8080/users -H 'Content-Type: application/json' -d '{}'

# 422 — CPF inválido + senhas diferentes
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

Regras da **empresa** (SKU, …) ficam na app (`v.Register`). Docs: [EN](../../../docs/guide/validation.md) · [PT](../../../docs/pt/guide/validation.md).
