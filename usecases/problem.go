package usecases

import (
	"encoding/json"
	"net/http"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// ProblemDetails is an ErrorHandler that renders errors as RFC 9457 / 7807
// "application/problem+json" documents:
//
//	app := nitro.New(nitro.Config{ErrorHandler: usecases.ProblemDetails()})
//
// HTTP errors keep their status; the "error" message becomes "detail" and any
// additional body fields (e.g. validation details) are preserved as extension
// members. Non-HTTP errors become a generic 500 — internals are never leaked.
func ProblemDetails() func(c *domain.Ctx, err error) error {
	return func(c *domain.Ctx, err error) error {
		status := http.StatusInternalServerError
		detail := ""
		ext := map[string]any{}
		if he, ok := err.(domain.HTTPError); ok {
			status = he.StatusCode()
			switch b := he.Body().(type) {
			case map[string]string:
				for k, v := range b {
					if k == "error" {
						detail = v
					} else {
						ext[k] = v
					}
				}
			case map[string]any:
				for k, v := range b {
					if k == "error" {
						if s, ok := v.(string); ok {
							detail = s
							continue
						}
					}
					ext[k] = v
				}
			default:
				if b != nil {
					ext["errors"] = b
				}
			}
		}
		if status >= 500 {
			detail = "" // never expose internal failure detail
		}
		doc := map[string]any{}
		for k, v := range ext {
			doc[k] = v
		}
		doc["type"] = "about:blank"
		doc["title"] = http.StatusText(status)
		doc["status"] = status
		if detail != "" {
			doc["detail"] = detail
		}
		doc["instance"] = c.Path()
		if rid := c.RequestID(); rid != "" {
			doc["request_id"] = rid
		}
		data, mErr := json.Marshal(doc)
		if mErr != nil {
			data = []byte(`{"type":"about:blank","title":"Internal Server Error","status":500}`)
			status = http.StatusInternalServerError
		}
		c.SetStatus(status)
		c.SetHeader("Content-Type", "application/problem+json")
		c.SetBody(data)
		return nil
	}
}
