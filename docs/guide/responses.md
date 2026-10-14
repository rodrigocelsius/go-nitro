# Responses — HTTP responses

**Language:** [English](responses.md) · [Português](../pt/guide/responses.md)

Success and payloads. Errors: [Errors](errors.md).

## JSON

```go
return c.OK(user)                                    // 200
return c.OK(nitro.Map{"key": "value"})
return c.Created(user)                               // 201
return c.Accepted(nitro.Map{"job_id": "abc"})        // 202
return c.NoContent()                                 // 204
return c.JSON(422, nitro.Map{"error": "invalid"})    // custom status
return c.Paginated(items, total, page, perPage)
// {"items":[...],"total":100,"page":1,"per_page":20,"total_pages":5}
```

## Text, bytes, redirect, file, stream

```go
return c.Text("ok")
return c.Text("created", 201)
return c.Bytes(200, data)
return c.Bytes(200, png, "image/png")
return c.Redirect("/login")          // 302
return c.Redirect("/login", 301)
return c.Attachment("report.pdf", pdfBytes)
return c.CSV("drivers.csv", []string{"name", "phone"}, rows) // rows [][]string
return c.File("report.csv", csvData)
return c.File("report.csv", csvData, "inline")
return c.Stream(fn)                  // SSE/CSV streaming — set Content-Type first if needed
```

**Do not** build CSV + `Fasthttp().SetBody` by hand — use `c.CSV` / `c.Attachment`.

## Pagination (helpers)

```go
page, perPage := c.Int("page", 1), c.Int("per_page", 20)
all := repo.GetAll()
p := nitro.Paginate(len(all), page, perPage)
items := nitro.PageSlice(all, p)
return c.OK(nitro.Map{
    "items": nitro.ToAny(items),
    "total": p.Total, "page": p.Page, "per_page": p.PerPage, "total_pages": p.TotalPages,
})
// or: return c.Paginated(items, total, page, perPage)
```

## Summary

| Method | Status |
|--------|--------|
| `c.OK` / `Created` / `Accepted` / `NoContent` | 200 / 201 / 202 / 204 |
| `c.JSON` / `Paginated` | custom / 200 |
| `c.Text` / `Bytes` / `Attachment` / `CSV` / `File` / `Stream` / `Redirect` | see signature |

## See also

- [Request](request.md) · [Errors](errors.md) · [Recipes](recipes.md)
