package fasthttp

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// ListenData holds server metadata at OnListen time.
type ListenData struct {
	Addr string
	Host string
	Port int
}

// Hooks groups application lifecycle callbacks.
type Hooks struct {
	onListen   []func(ListenData) error
	onShutdown []func() error
	onRoute    []func(domain.RouteInfo) error
}

// Hooks returns the App hook registry (created on demand).
func (a *App) Hooks() *Hooks {
	if a.hooks == nil {
		a.hooks = &Hooks{}
	}
	return a.hooks
}

// OnListen registers a callback run immediately before the server accepts connections.
func (h *Hooks) OnListen(fn func(ListenData) error) *Hooks {
	if fn != nil {
		h.onListen = append(h.onListen, fn)
	}
	return h
}

// OnShutdown registers a callback run after the server stops.
func (h *Hooks) OnShutdown(fn func() error) *Hooks {
	if fn != nil {
		h.onShutdown = append(h.onShutdown, fn)
	}
	return h
}

// OnRoute registers a callback run when a route is registered.
func (h *Hooks) OnRoute(fn func(domain.RouteInfo) error) *Hooks {
	if fn != nil {
		h.onRoute = append(h.onRoute, fn)
	}
	return h
}

func (a *App) fireOnListen(addr string) error {
	h := a.hooks
	if h == nil || len(h.onListen) == 0 {
		return nil
	}
	data := parseListenData(addr)
	for _, fn := range h.onListen {
		if err := fn(data); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) fireOnShutdown() {
	h := a.hooks
	if !a.disableStartupMessage {
		fmt.Fprintln(os.Stdout, "  Nitro: shutting down…")
	}
	if h == nil {
		if !a.disableStartupMessage {
			fmt.Fprintln(os.Stdout, "  Nitro: server stopped")
		}
		return
	}
	for _, fn := range h.onShutdown {
		_ = fn()
	}
	if !a.disableStartupMessage {
		fmt.Fprintln(os.Stdout, "  Nitro: server stopped")
	}
}

func (a *App) fireOnRoute(info domain.RouteInfo) {
	h := a.hooks
	if h == nil || len(h.onRoute) == 0 {
		return
	}
	for _, fn := range h.onRoute {
		_ = fn(info)
	}
}

func parseListenData(addr string) ListenData {
	data := ListenData{Addr: addr, Host: addr}
	host, port, ok := strings.Cut(addr, ":")
	if !ok {
		return data
	}
	// "[::1]:8080" or ":8080"
	if strings.HasPrefix(addr, "[") {
		if i := strings.LastIndex(addr, "]:"); i >= 0 {
			data.Host = addr[1:i]
			port = addr[i+2:]
		}
	} else {
		data.Host = host
	}
	if p, err := strconv.Atoi(port); err == nil {
		data.Port = p
	}
	return data
}
