# Transações e hooks de escrita

**Idioma:** [Português](transactions.md) · [English](../../guide/transactions.md)

As transações do Nitro são **ambientes**: a transação viaja no `context.Context`.
Qualquer `Store` do BLinq, repositório `RepositoryFor` ou `app.Resource` chamado
com esse `ctx` participa dela. Sem handle de transação para repassar, sem um
segundo conjunto de repositórios.

| Necessidade | API |
|-------------|-----|
| Várias escritas, tudo ou nada | `db.InTx(ctx, func(ctx) error)` · `store.InTx` · `nitro.InTx(ctx, db, fn)` |
| Retentar em deadlock / falha de serialização | `db.InTxRetry(ctx, 3, fn)` · `sqladapter.WithRetry` |
| Efeito colateral só se os dados foram salvos (fila, e-mail, webhook) | `nitro.AfterCommit(ctx, fn)` · `ResourceConfig.AfterCommit` |
| Compensação / alerta no rollback | `nitro.AfterRollback(ctx, fn)` |
| Validar / preparar **antes** de uma escrita do Resource | `BeforeWrite`, `BeforeCreate`, `BeforeUpdate`, `BeforeDelete` |
| Gravar linhas relacionadas **depois** de uma escrita do Resource | `AfterCreate`, `AfterUpdate`, `AfterDelete`, `OnMutate` |
| Ação de negócio com vários passos | `ResourceConfig.Commands` (`nitro.Cmd`) |
| Cabeçalho + itens (venda + itens) com corpo aninhado | `sqladapter.Related` |
| Este código está dentro de uma transação? | `nitro.InTransaction(ctx)` |

## Convenção: escritas do Resource são atômicas

Quando o repositório suporta transações (todo `Store` / `StoreFor` /
`RepositoryFor` do BLinq) **e** o Resource declara um hook de escrita
(`BeforeWrite`, `Before*`, `After*`, `OnMutate`) ou `Commands`, cada escrita roda
como **uma transação**:

```
BEGIN
  load (PATCH/DELETE)  → BeforeWrite → BeforeCreate|BeforeUpdate|BeforeDelete
  INSERT|UPDATE|DELETE
  AfterCreate|AfterUpdate|AfterDelete → OnMutate
COMMIT  → AfterCommit (fora da tx) → resposta HTTP
```

Qualquer hook que retorne erro desfaz tudo e o cliente recebe esse erro (ex.:
`nitro.Conflict(...)` → 409). Conflitos transitórios do banco (deadlock / falha
de serialização) respondem **409 `{"code":"tx_conflict","retryable":true}`**
para o cliente retentar. Resources sem hooks mantêm o caminho de um único
comando (sem `BEGIN`/`COMMIT` extras). Para desligar: `ResourceConfig{NoTx: true}`.

### Exemplo 1 — venda + detalhes da venda

Inserir uma venda precisa inserir as linhas de detalhe com o id da venda;
qualquer erro desfaz as duas.

```go
details := sqladapter.StoreFor[model.SaleDetail](db)

app.Resource(&model.Sale{}, sqladapter.StoreFor[model.Sale](db), &nitro.ResourceConfig{
    AfterCreate: func(ctx context.Context, e any) error {
        sale := e.(*model.Sale)
        for i := range sale.Items {            // Items []SaleDetail `db:"-"`
            sale.Items[i].SaleID = sale.ID
        }
        _, err := details.CreateMany(ctx, sale.Items) // mesma tx (ctx)
        return err
    },
    AfterCommit: func(ctx context.Context, action string, e any) {
        _ = nitro.Publish(ctx, queue, "sale.created", e) // só depois do COMMIT
    },
})
```

Para o formato comum cabeçalho + itens, `sqladapter.Related` faz isso (mais
`GET/POST/PATCH /sales/:id/sale_details` e um hook de recálculo) sem código:
veja [business-resource.md](../../guide/business-resource.md).

### Exemplo 2 — conta paga libera o cliente

```go
customers := sqladapter.StoreFor[model.Customer](db)

app.Resource(&model.Payable{}, sqladapter.StoreFor[model.Payable](db), &nitro.ResourceConfig{
    AfterCreate: func(ctx context.Context, e any) error {
        p := e.(*model.Payable)
        if !p.Paid {                       // ação condicional
            return nil
        }
        _, err := customers.Update(ctx, p.CustomerID, modelgen.Customers.Status.Set("clear"))
        return err                         // erro → insert da conta desfeito
    },
})
```

### Verificações prévias e ramificações condicionais

Hooks são Go puro, então condições são `if`/`switch`:

```go
BeforeDelete: func(ctx context.Context, id any) error {
    open, err := payables.CountWhere(ctx, modelgen.Payables.CustomerID.Eq(id), modelgen.Payables.Paid.Eq(false))
    if err != nil { return err }
    if open > 0 { return nitro.Conflict("customer has open payables") }
    return nil
},
BeforeWrite: func(ctx context.Context, e any) error {
    o := e.(*model.Order)
    switch {
    case o.Total > 10_000:
        o.Status = "review"            // uma ação…
    default:
        o.Status = "approved"          // …ou outra
    }
    return nil
},
```

## Casos de uso e serviços (fora do Resource)

```go
err := db.InTx(ctx, func(ctx context.Context) error {
    sale, err := sales.CreateOne(ctx, &model.Sale{CustomerID: cid})
    if err != nil {
        return err
    }
    if _, err := stock.UpdateWhere(ctx, stock.Query().Where(modelgen.Stock.SKU.Eq(sku)),
        modelgen.Stock.Qty.Inc(-1)); err != nil {
        return err
    }
    nitro.AfterCommit(ctx, func(ctx context.Context) { notify(sale) })
    return nil
})
```

- `InTx` **aninhado** (ou um hook do Resource chamando um caso de uso que abre `InTx`) no mesmo banco **participa** da transação externa; a mais externa faz o commit.
- Stores de **outro** banco não são capturados pela transação (cada `DB` tem a sua).
- `db.Tx(ctx, func(tx *sqladapter.DB) error)` continua funcionando: entrega um `*DB` preso à transação para `StoreOf(tx, …)` funcionar com qualquer `ctx`.
- Retente a unidade de trabalho inteira com `db.InTxRetry(ctx, tentativas, fn)`; mantenha efeitos colaterais em `AfterCommit` para que a retentativa não os repita.
- Locks de linha dentro da tx: `query.ForUpdate()` / `SkipLocked()` (veja [blinq.md](blinq.md)).
- O cache de SELECT é ignorado dentro da transação e invalidado de novo depois do `COMMIT`.

## Fora de uma transação

`AfterCommit` chamado sem transação roda na hora, e `AfterRollback` não faz
nada — o mesmo código de hook funciona com ou sem `NoTx` e com repositórios em
memória (que não suportam transações).
