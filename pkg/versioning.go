package nitro

import (
	"net/http"
	"time"
)

// Deprecated marks every route it guards as deprecated using the standard
// headers clients and gateways understand: Deprecation (RFC 9745), Sunset
// (RFC 8594) and, when successor is non-empty, Link rel="successor-version".
// A zero sunset omits the Sunset header.
//
//	v1 := app.Group("/v1", nitro.Deprecated(time.Date(2027, 6, 30, 0, 0, 0, 0, time.UTC), "/v2"))
func Deprecated(sunset time.Time, successor string) Handler {
	var sunsetVal string
	if !sunset.IsZero() {
		sunsetVal = sunset.UTC().Format(http.TimeFormat)
	}
	link := ""
	if successor != "" {
		link = "<" + successor + `>; rel="successor-version"`
	}
	return func(c *Ctx) error {
		c.SetHeader("Deprecation", "true")
		if sunsetVal != "" {
			c.SetHeader("Sunset", sunsetVal)
		}
		if link != "" {
			c.SetHeader("Link", link)
		}
		return nil
	}
}
