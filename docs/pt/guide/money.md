# Money (`nitro.Money`)

**Idioma:** [Português](money.md) · [English](../../guide/money.md)

Valor em **unidades menores** (centavos) + moeda ISO 4217.

```go
price := nitro.MoneyFromMajor(19.90, "BRL")
nitro.FormatMoney(price, "pt-BR") // "19,90 BRL"
m, _ := nitro.ParseMoney("1.234,56 BRL", "", "pt-BR")
```

Codegen expande `Fee nitro.Money \`db:"fee"\`` → `fee_amount BIGINT` + `fee_currency CHAR(3)`.

Câmbio: implemente `money.FXProvider` e use `money.ConvertCurrency`.

Tag DTO: `transform:"money=BRL,pt-BR"`.
