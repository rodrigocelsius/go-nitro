package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/gofiber/fiber/v3"
	fibermw "github.com/gofiber/fiber/v3/middleware/logger"
	fiberrec "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/labstack/echo/v5"
	echomw "github.com/labstack/echo/v5/middleware"
	"github.com/valyala/fasthttp"

	nf "github.com/rodrigocelsius/go-nitro/adapters/fasthttp/v2"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}
type In struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	fw := flag.String("fw", "nitro", "framework")
	addr := flag.String("addr", "127.0.0.1:9000", "addr")
	flag.Parse()
	log.SetOutput(io.Discard)
	switch *fw {
	case "nitro", "nitro-bare", "nitro-defaults":
		// nitro        = New() as shipped (security headers + request id on)
		// nitro-bare   = those two disabled: framework core vs fasthttp/Fiber
		// nitro-defaults = nitro.New() full stack (adds Recover, Logger, MaxBody)
		cfg := domain.Config{}
		if *fw == "nitro-bare" {
			cfg = domain.Config{DisableRequestID: true, Security: domain.SecurityConfig{DisableHeaders: true}}
		}
		app := nf.New(cfg)
		if *fw == "nitro-defaults" {
			// what nitro.New() enables (without OTel): Recover, Logger, MaxBody, RequestID, SecurityHeaders
			app.Use(nf.RecoverMiddlewareWithDebug(false))
			pre, post := nf.Logger(nf.LoggerConfig{SkipRoutes: nf.DefaultSkipRoutes})
			app.Use(pre)
			app.UsePost(post)
			app.Use(nf.MaxBodySizeMiddleware(1 << 20))
			app.Use(nf.RequestIDMiddleware())
			app.Use(nf.SecurityHeadersMiddleware())
		}
		app.Get("/ping", func(c *domain.Ctx) error { return c.AdapterData.(*nf.FastCtx).SendString("pong") })
		app.Get("/users/:id", func(c *domain.Ctx) error {
			fc := c.AdapterData.(*nf.FastCtx)
			return fc.JSON(200, User{ID: fc.Param("id"), Name: "Ana", Email: "ana@x.com"})
		})
		app.Post("/echo", func(c *domain.Ctx) error {
			fc := c.AdapterData.(*nf.FastCtx)
			var in In
			if err := fc.BodyJSON(&in); err != nil {
				return err
			}
			return fc.JSON(200, in)
		})
		log.Fatal(app.Listen(*addr))
	case "fasthttp":
		log.Fatal(fasthttp.ListenAndServe(*addr, func(c *fasthttp.RequestCtx) {
			switch string(c.Path()) {
			case "/ping":
				c.SetBodyString("pong")
			case "/echo":
				var in In
				_ = json.Unmarshal(c.PostBody(), &in)
				c.SetContentType("application/json")
				b, _ := json.Marshal(in)
				c.SetBody(b)
			default:
				id := string(c.Path()[len("/users/"):])
				c.SetContentType("application/json")
				b, _ := json.Marshal(User{ID: id, Name: "Ana", Email: "ana@x.com"})
				c.SetBody(b)
			}
		}))
	case "fiber", "fiber-defaults":
		app := fiber.New()
		if *fw == "fiber-defaults" {
			app.Use(fiberrec.New())
			app.Use(fibermw.New(fibermw.Config{Stream: io.Discard}))
		}
		app.Get("/ping", func(c fiber.Ctx) error { return c.SendString("pong") })
		app.Get("/users/:id", func(c fiber.Ctx) error {
			return c.JSON(User{ID: c.Params("id"), Name: "Ana", Email: "ana@x.com"})
		})
		app.Post("/echo", func(c fiber.Ctx) error {
			var in In
			if err := c.Bind().JSON(&in); err != nil {
				return err
			}
			return c.JSON(in)
		})
		log.Fatal(app.Listen(*addr, fiber.ListenConfig{DisableStartupMessage: true}))
	case "gin", "gin-defaults":
		gin.SetMode(gin.ReleaseMode)
		var r *gin.Engine
		if *fw == "gin-defaults" {
			gin.DefaultWriter = io.Discard
			r = gin.Default()
		} else {
			r = gin.New()
		}
		r.GET("/ping", func(c *gin.Context) { c.String(200, "pong") })
		r.GET("/users/:id", func(c *gin.Context) { c.JSON(200, User{ID: c.Param("id"), Name: "Ana", Email: "ana@x.com"}) })
		r.POST("/echo", func(c *gin.Context) {
			var in In
			if err := c.ShouldBindJSON(&in); err != nil {
				c.AbortWithStatus(400)
				return
			}
			c.JSON(200, in)
		})
		log.Fatal(r.Run(*addr))
	case "echo", "echo-defaults":
		e := echo.New()
		if *fw == "echo-defaults" {
			e.Use(echomw.Recover())
			e.Use(echomw.RequestLogger())
		}
		e.GET("/ping", func(c *echo.Context) error { return c.String(200, "pong") })
		e.GET("/users/:id", func(c *echo.Context) error {
			return c.JSON(200, User{ID: c.Param("id"), Name: "Ana", Email: "ana@x.com"})
		})
		e.POST("/echo", func(c *echo.Context) error {
			var in In
			if err := c.Bind(&in); err != nil {
				return err
			}
			return c.JSON(200, in)
		})
		sc := echo.StartConfig{Address: *addr, HideBanner: true, HidePort: true}
		log.Fatal(sc.Start(context.Background(), e))
	case "chi":
		r := chi.NewRouter()
		_ = chimw.Recoverer
		r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("pong")) })
		r.Get("/users/{id}", func(w http.ResponseWriter, q *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(User{ID: chi.URLParam(q, "id"), Name: "Ana", Email: "ana@x.com"})
		})
		r.Post("/echo", func(w http.ResponseWriter, q *http.Request) {
			var in In
			if json.NewDecoder(q.Body).Decode(&in) != nil {
				w.WriteHeader(400)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(in)
		})
		log.Fatal(http.ListenAndServe(*addr, r))
	case "nethttp":
		m := http.NewServeMux()
		m.HandleFunc("GET /ping", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("pong")) })
		m.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, q *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(User{ID: q.PathValue("id"), Name: "Ana", Email: "ana@x.com"})
		})
		m.HandleFunc("POST /echo", func(w http.ResponseWriter, q *http.Request) {
			var in In
			if json.NewDecoder(q.Body).Decode(&in) != nil {
				w.WriteHeader(400)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(in)
		})
		log.Fatal(http.ListenAndServe(*addr, m))
	default:
		os.Exit(2)
	}
}
