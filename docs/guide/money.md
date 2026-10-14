# Money (`nitro.Money`)

**Language:** [English](money.md) · [Português](../pt/guide/money.md)

Value in **minor units** (cents) + ISO 4217 currency.

```go
price := nitro.MoneyFromMajor(19.90, "BRL")
nitro.FormatMoney(price, "pt-BR") // "19,90 BRL"
m, _ := nitro.ParseMoney("1.234,56 BRL", "", "pt-BR")
```

Codegen expands `Fee nitro.Money \`db:"fee"\`` → `fee_amount BIGINT` + `fee_currency CHAR(3)`.

FX: implement `money.FXProvider` and use `money.ConvertCurrency`.

DTO tag: `transform:"money=BRL,pt-BR"`.
