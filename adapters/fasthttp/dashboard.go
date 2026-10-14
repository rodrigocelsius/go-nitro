package fasthttp

import (
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/rodrigocelsius/go-nitro/codegen/openapi/v2"
	"github.com/rodrigocelsius/go-nitro/domain/v2"
)

// DashboardConfig configures the dev dashboard.
type DashboardConfig struct {
	Enabled   bool
	BasePath  string
	Generator *openapi.Generator
	// Middleware protects every dashboard route (e.g. IPWhitelistMiddleware or
	// JWT + RequireRole). REQUIRED when the app runs with Env == EnvProduction:
	// RegisterDashboard panics otherwise, because the dashboard reveals route
	// topology and runtime internals.
	Middleware []domain.Handler
}

// DashboardData is data exposed by the dashboard.
type DashboardData struct {
	Routes        []domain.RouteInfo  `json:"routes"`
	Uptime        string              `json:"uptime"`
	GoVersion     string              `json:"goVersion"`
	NumCPU        int                 `json:"numCPU"`
	NumGoroutines int                 `json:"numGoroutines"`
	Memory        MemoryStats         `json:"memory"`
	Middleware    []MiddlewareMetrics `json:"middleware,omitempty"`
	Requests      RequestStats        `json:"requests"`
}

// MemoryStats is detailed memory statistics.
type MemoryStats struct {
	Alloc        uint64 `json:"alloc"`
	TotalAlloc   uint64 `json:"totalAlloc"`
	Sys          uint64 `json:"sys"`
	NumGC        uint32 `json:"numGC"`
	PauseTotalNs int64  `json:"pauseTotalNs"`
	NextGC       uint64 `json:"nextGC"`
	Mallocs      uint64 `json:"mallocs"`
	Frees        uint64 `json:"frees"`
	HeapAlloc    uint64 `json:"heapAlloc"`
	HeapSys      uint64 `json:"heapSys"`
	HeapIdle     uint64 `json:"heapIdle"`
	HeapInuse    uint64 `json:"heapInuse"`
	HeapReleased uint64 `json:"heapReleased"`
	HeapObjects  uint64 `json:"heapObjects"`
	StackSys     uint64 `json:"stackSys"`
	StackInuse   uint64 `json:"stackInuse"`
	MstatsSys    uint64 `json:"mstatsSys"`
}

var startTime = time.Now()

// DashboardHandler returns dashboard HTML.
func DashboardHandler(cfg DashboardConfig) domain.Handler {
	return func(c *domain.Ctx) error {
		c.SetBody([]byte(dashboardHTML))
		return nil
	}
}

// DashboardDataHandler returns dashboard JSON data.
func DashboardDataHandler(app *App, cfg DashboardConfig) domain.Handler {
	return func(c *domain.Ctx) error {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)

		data := DashboardData{
			Routes:        app.Routes(),
			Uptime:        time.Since(startTime).String(),
			GoVersion:     runtime.Version(),
			NumCPU:        runtime.NumCPU(),
			NumGoroutines: runtime.NumGoroutine(),
			Memory: MemoryStats{
				Alloc:        ms.Alloc,
				TotalAlloc:   ms.TotalAlloc,
				Sys:          ms.Sys,
				NumGC:        ms.NumGC,
				PauseTotalNs: int64(ms.PauseTotalNs),
				NextGC:       ms.NextGC,
				Mallocs:      ms.Mallocs,
				Frees:        ms.Frees,
				HeapAlloc:    ms.HeapAlloc,
				HeapSys:      ms.HeapSys,
				HeapIdle:     ms.HeapIdle,
				HeapInuse:    ms.HeapInuse,
				HeapReleased: ms.HeapReleased,
				HeapObjects:  ms.HeapObjects,
				StackSys:     ms.StackSys,
				StackInuse:   ms.StackInuse,
				MstatsSys:    ms.Sys,
			},
		}

		if app.metrics != nil {
			data.Middleware = app.metrics.Metrics()
			data.Requests = app.metrics.GetRequestStats()
		}

		if fc, ok := c.AdapterData.(*FastCtx); ok {
			return fc.JSON(200, data)
		}
		jsonData, err := json.Marshal(data)
		if err != nil {
			return err
		}
		c.SetBody(jsonData)
		c.SetHeader("Content-Type", "application/json")
		return nil
	}
}

// RegisterDashboard registers dashboard routes.
func RegisterDashboard(app *App, cfg DashboardConfig) {
	if !cfg.Enabled {
		return
	}
	base := cfg.BasePath
	if base == "" {
		base = "/nitro"
	}

	// The dashboard exposes internal route topology and runtime metrics
	// (CWE-200). In production it must be protected; see vuln.md (V-08).
	if app.env == domain.EnvProduction && len(cfg.Middleware) == 0 {
		app.AddConfigError(fmt.Errorf("nitro: RegisterDashboard in production requires DashboardConfig.Middleware (e.g. IPWhitelistMiddleware or JWT+RequireRole) — refusing to expose %s/dashboard unauthenticated", base))
		return
	}

	app.Get(base+"/dashboard", DashboardHandler(cfg), cfg.Middleware...)
	app.Get(base+"/api/dashboard", DashboardDataHandler(app, cfg), cfg.Middleware...)
	if cfg.Generator != nil {
		app.Get(base+"/openapi.json", OpenAPIHandler(cfg.Generator), cfg.Middleware...)
	}
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Nitro Dashboard</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #0f172a; color: #e2e8f0; padding: 20px; }
        .header { text-align: center; margin-bottom: 30px; }
        .header h1 { font-size: 2.5rem; color: #38bdf8; }
        .header p { color: #94a3b8; margin-top: 5px; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; max-width: 1200px; margin: 0 auto; }
        .card { background: #1e293b; border-radius: 12px; padding: 20px; border: 1px solid #334155; }
        .card h2 { color: #38bdf8; font-size: 1.2rem; margin-bottom: 15px; }
        .stat { display: flex; justify-content: space-between; padding: 8px 0; border-bottom: 1px solid #334155; }
        .stat:last-child { border-bottom: none; }
        .stat-label { color: #94a3b8; }
        .stat-value { color: #e2e8f0; font-weight: 600; }
        .route { background: #0f172a; padding: 10px; border-radius: 8px; margin-bottom: 8px; font-family: monospace; }
        .method { color: #fff; padding: 2px 8px; border-radius: 4px; font-size: 0.8rem; margin-right: 10px; }
        .method-GET { background: #22c55e; }
        .method-POST { background: #3b82f6; }
        .method-PUT { background: #f59e0b; }
        .method-DELETE { background: #ef4444; }
        .footer { text-align: center; margin-top: 30px; color: #64748b; font-size: 0.9rem; }
        .live { display: inline-block; width: 8px; height: 8px; background: #22c55e; border-radius: 50%; margin-right: 5px; animation: pulse 2s infinite; }
        @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.5; } }
        .error { color: #ef4444; }
        .success { color: #22c55e; }
    </style>
</head>
<body>
    <div class="header">
        <h1>⚡ Nitro Dashboard</h1>
        <p><span class="live"></span>Development Server Running</p>
    </div>
    <div class="grid">
        <div class="card">
            <h2>📊 Server Info</h2>
            <div id="serverInfo">Loading...</div>
        </div>
        <div class="card">
            <h2>📈 Request Stats</h2>
            <div id="requests">Loading...</div>
        </div>
        <div class="card">
            <h2>💾 Memory</h2>
            <div id="memory">Loading...</div>
        </div>
        <div class="card">
            <h2>🔄 Runtime</h2>
            <div id="runtime">Loading...</div>
        </div>
        <div class="card" style="grid-column: 1 / -1;">
            <h2>🛣️ Registered Routes</h2>
            <div id="routes">Loading...</div>
        </div>
        <div class="card" style="grid-column: 1 / -1;">
            <h2>⏱️ Middleware Timing</h2>
            <div id="middleware">Loading...</div>
        </div>
    </div>
    <div class="footer">
        Nitro HTTP Framework • <a href="/nitro/openapi.json" style="color: #38bdf8;">OpenAPI Spec</a>
    </div>
    <script>
        async function loadDashboard() {
            try {
                const res = await fetch('/nitro/api/dashboard');
                const data = await res.json();

                document.getElementById('serverInfo').innerHTML = [
                    stat('Go Version', data.goVersion),
                    stat('CPUs', data.numCPU),
                    stat('Uptime', data.uptime),
                ].join('');

                var reqHtml = '';
                if (data.requests) {
                    var r = data.requests;
                    reqHtml = [
                        stat('Total Requests', r.totalRequests),
                        stat('Success', '<span class="success">' + r.successCount + '</span>'),
                        stat('Errors', '<span class="error">' + r.errorCount + '</span>'),
                        stat('Error Rate', r.errorRate.toFixed(2) + '%'),
                        stat('Requests/sec', r.requestsPerSec.toFixed(2)),
                        stat('Avg Latency', formatNs(r.avgLatencyNs)),
                        stat('P50 Latency', formatNs(r.p50LatencyNs)),
                        stat('P95 Latency', formatNs(r.p95LatencyNs)),
                        stat('P99 Latency', formatNs(r.p99LatencyNs)),
                    ].join('');
                } else {
                    reqHtml = '<p style="color:#64748b;">Enable metrics to see request stats</p>';
                }
                document.getElementById('requests').innerHTML = reqHtml;

                document.getElementById('memory').innerHTML = [
                    stat('Heap Alloc', formatBytes(data.memory.heapAlloc)),
                    stat('Heap Sys', formatBytes(data.memory.heapSys)),
                    stat('Heap In Use', formatBytes(data.memory.heapInuse)),
                    stat('Heap Idle', formatBytes(data.memory.heapIdle)),
                    stat('Heap Objects', data.memory.heapObjects.toLocaleString()),
                    stat('Total Alloc', formatBytes(data.memory.totalAlloc)),
                    stat('Sys', formatBytes(data.memory.sys)),
                    stat('GC Cycles', data.memory.numGC),
                    stat('Pause Total', formatNs(data.memory.pauseTotalNs)),
                ].join('');

                document.getElementById('runtime').innerHTML = [
                    stat('Goroutines', data.numGoroutines),
                    stat('Stack In Use', formatBytes(data.memory.stackInuse)),
                    stat('Stack Sys', formatBytes(data.memory.stackSys)),
                    stat('Mallocs', data.memory.mallocs.toLocaleString()),
                    stat('Frees', data.memory.frees.toLocaleString()),
                    stat('Next GC', formatBytes(data.memory.nextGC)),
                ].join('');

                var mwHtml = '';
                if (data.middleware && data.middleware.length > 0) {
                    mwHtml = data.middleware.map(function(m) {
                        return '<div class="stat">' +
                            '<span class="stat-label">' + m.name + '</span>' +
                            '<span class="stat-value">' + formatNs(m.avgNs) + ' avg (' + m.count + ' calls)</span>' +
                            '</div>';
                    }).join('');
                } else {
                    mwHtml = '<p style="color:#64748b;">No metrics collected</p>';
                }
                document.getElementById('middleware').innerHTML = mwHtml;

                document.getElementById('routes').innerHTML = data.routes.map(r =>
                    '<div class="route">' +
                    '<span class="method method-' + r.method + '">' + r.method + '</span>' +
                    r.path +
                    (r.name ? ' <span style="color:#64748b;">(' + r.name + ')</span>' : '') +
                    '</div>'
                ).join('') || '<p style="color:#64748b;">No routes registered</p>';
            } catch (e) {
                console.error('Dashboard load failed:', e);
            }
        }

        function stat(label, value) {
            return '<div class="stat"><span class="stat-label">' + label + '</span><span class="stat-value">' + value + '</span></div>';
        }

        function formatBytes(bytes) {
            if (bytes < 1024) return bytes + ' B';
            if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
            return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
        }

        function formatNs(ns) {
            if (ns < 1000) return ns + ' ns';
            if (ns < 1000000) return (ns / 1000).toFixed(2) + ' µs';
            if (ns < 1000000000) return (ns / 1000000).toFixed(2) + ' ms';
            return (ns / 1000000000).toFixed(3) + ' s';
        }

        loadDashboard();
        setInterval(loadDashboard, 5000);
    </script>
</body>
</html>`
