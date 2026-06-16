package mux

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/trae-framework/vine/ctx"
	"github.com/trae-framework/vine/errors"
	"github.com/trae-framework/vine/middleware"
	"github.com/trae-framework/vine/radix"
)

type ErrorHandlerFunc func(*ctx.Ctx, error)

type routeEntry struct {
	Handler     ctx.HandlerFunc
	Middlewares []ctx.HandlerFunc
}

type recentError struct {
	Time    time.Time `json:"time"`
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Path    string    `json:"path"`
	Method  string    `json:"method"`
}

type Metrics struct {
	RouteCount      int            `json:"route_count"`
	TotalRequests   uint64         `json:"total_requests"`
	StatusCounts    map[string]int `json:"status_counts"`
	RecentErrors    []recentError  `json:"recent_errors"`
	UptimeSeconds   float64        `json:"uptime_seconds"`
	StartedAt       time.Time      `json:"started_at"`
}

const recentErrorsLimit = 20

type Router struct {
	trees        map[string]*radix.Tree
	middleware   []ctx.HandlerFunc
	notFound     ctx.HandlerFunc
	errorHandler ErrorHandlerFunc

	metricsMu       sync.Mutex
	startedAt       time.Time
	totalRequests   uint64
	statusCounts    map[string]int
	recentErrorsBuf []recentError
	recentErrorsPtr int
}

func defaultErrorHandler(c *ctx.Ctx, err error) {
	appErr := errors.FromError(err)
	c.JSON(appErr.Code, appErr)
}

func New() *Router {
	r := &Router{
		trees: make(map[string]*radix.Tree),
		notFound: func(c *ctx.Ctx) {
			c.NotFound("page not found")
		},
		errorHandler:    defaultErrorHandler,
		statusCounts:    make(map[string]int),
		recentErrorsBuf: make([]recentError, 0, recentErrorsLimit),
		startedAt:       time.Now(),
	}
	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"} {
		r.trees[method] = radix.NewTree()
	}
	return r
}

func funcName(fn interface{}) string {
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		return "?"
	}
	ptr := v.Pointer()
	fullName := runtime.FuncForPC(ptr).Name()
	if fullName == "" {
		return "closure"
	}
	slash := strings.LastIndex(fullName, "/")
	if slash >= 0 {
		fullName = fullName[slash+1:]
	}
	dot := strings.Index(fullName, ".")
	if dot >= 0 {
		fullName = fullName[dot+1:]
	}
	fullName = strings.TrimSuffix(fullName, "·1")
	fullName = strings.TrimSuffix(fullName, "·2")
	return fullName
}

func handlerNames(handlers []ctx.HandlerFunc) []string {
	names := make([]string, len(handlers))
	for i, h := range handlers {
		names[i] = funcName(h)
	}
	return names
}

func (r *Router) SetErrorHandler(h ErrorHandlerFunc) {
	r.errorHandler = h
}

func (r *Router) Use(mw ...ctx.HandlerFunc) {
	r.middleware = append(r.middleware, mw...)
}

func (r *Router) GET(path string, handler ctx.HandlerFunc) {
	r.addRoute("GET", path, handler, nil)
}

func (r *Router) POST(path string, handler ctx.HandlerFunc) {
	r.addRoute("POST", path, handler, nil)
}

func (r *Router) PUT(path string, handler ctx.HandlerFunc) {
	r.addRoute("PUT", path, handler, nil)
}

func (r *Router) DELETE(path string, handler ctx.HandlerFunc) {
	r.addRoute("DELETE", path, handler, nil)
}

func (r *Router) PATCH(path string, handler ctx.HandlerFunc) {
	r.addRoute("PATCH", path, handler, nil)
}

func (r *Router) HEAD(path string, handler ctx.HandlerFunc) {
	r.addRoute("HEAD", path, handler, nil)
}

func (r *Router) OPTIONS(path string, handler ctx.HandlerFunc) {
	r.addRoute("OPTIONS", path, handler, nil)
}

func (r *Router) Group(prefix string, handlers ...ctx.HandlerFunc) *Group {
	return &Group{
		prefix:      prefix,
		router:      r,
		middlewares: append([]ctx.HandlerFunc{}, handlers...),
	}
}

func (r *Router) addRoute(method, path string, handler ctx.HandlerFunc, middlewares []ctx.HandlerFunc) {
	tree, ok := r.trees[method]
	if !ok {
		tree = radix.NewTree()
		r.trees[method] = tree
	}
	entry := &routeEntry{
		Handler:     handler,
		Middlewares: append([]ctx.HandlerFunc{}, middlewares...),
	}
	tree.Add(method, path, entry)
}

func (r *Router) SetNotFound(handler ctx.HandlerFunc) {
	r.notFound = handler
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	origMethod := req.Method
	path := req.URL.Path

	c := ctx.New(w, req)
	defer c.Release()

	c.SetErrorHandler(func(c *ctx.Ctx, err error) {
		appErr := errors.FromError(err)
		r.recordError(appErr.Code, appErr.Message, path, origMethod)
		r.errorHandler(c, err)
	})

	var headFallback bool
	targetMethod := origMethod

	if origMethod == "HEAD" {
		if _, hasTree := r.trees["HEAD"]; hasTree {
			if tree := r.trees["HEAD"]; tree != nil {
				if res := tree.Get("HEAD", path); !res.Found {
					targetMethod = "GET"
					headFallback = true
				}
			} else {
				targetMethod = "GET"
				headFallback = true
			}
		}
	}

	useMethod := targetMethod
	tree, ok := r.trees[useMethod]
	if !ok {
		if origMethod == "OPTIONS" {
			r.handleOptionsFallback(c)
			return
		}
		r.notFound(c)
		r.recordStatus(c.StatusCode())
		return
	}

	result := tree.Get(useMethod, path)
	if !result.Found {
		if origMethod == "OPTIONS" && useMethod == "OPTIONS" {
			r.handleOptionsFallback(c)
			return
		}
		if headFallback {
		}
		if result.TSR {
			c.JSON(http.StatusMovedPermanently, map[string]string{
				"redirect": result.TSRPath,
			})
			r.recordStatus(c.StatusCode())
			return
		}
		r.notFound(c)
		r.recordStatus(c.StatusCode())
		return
	}

	entry, ok := result.Handler.(*routeEntry)
	if !ok {
		r.notFound(c)
		r.recordStatus(c.StatusCode())
		return
	}

	c.SetParams(result.Params)

	handlers := make([]ctx.HandlerFunc, 0, len(r.middleware)+len(entry.Middlewares)+1)
	handlers = append(handlers, r.middleware...)
	handlers = append(handlers, entry.Middlewares...)
	handlers = append(handlers, entry.Handler)

	c.SetHandlers(handlers)
	atomic.AddUint64(&r.totalRequests, 1)

	if headFallback {
		cw := &captureWriter{inner: w}
		c.Writer = cw
		c.Next()
		status := cw.status
		if status == 0 {
			status = c.StatusCode()
		}
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		r.recordStatus(status)
		return
	}

	c.Next()
	r.recordStatus(c.StatusCode())
}

func (r *Router) handleOptionsFallback(c *ctx.Ctx) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,HEAD,OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
	c.Header("Access-Control-Max-Age", "86400")
	c.Status(http.StatusNoContent)
	c.Writer.WriteHeader(http.StatusNoContent)
	atomic.AddUint64(&r.totalRequests, 1)
	r.recordStatus(http.StatusNoContent)
}

type captureWriter struct {
	inner  http.ResponseWriter
	header http.Header
	status int
}

func (cw *captureWriter) Header() http.Header {
	if cw.header == nil {
		cw.header = make(http.Header)
		for k, v := range cw.inner.Header() {
			nv := make([]string, len(v))
			copy(nv, v)
			cw.header[k] = nv
		}
	}
	return cw.header
}

func (cw *captureWriter) Write(data []byte) (int, error) {
	return len(data), nil
}

func (cw *captureWriter) WriteHeader(code int) {
	cw.status = code
	for k, v := range cw.header {
		for _, val := range v {
			cw.inner.Header().Set(k, val)
		}
	}
}

func (r *Router) recordStatus(code int) {
	if code == 0 {
		code = 200
	}
	key := fmt.Sprintf("%d", code)
	r.metricsMu.Lock()
	r.statusCounts[key]++
	r.metricsMu.Unlock()
}

func (r *Router) recordError(code int, msg, path, method string) {
	r.metricsMu.Lock()
	if len(r.recentErrorsBuf) < recentErrorsLimit {
		r.recentErrorsBuf = append(r.recentErrorsBuf, recentError{})
	}
	r.recentErrorsBuf[r.recentErrorsPtr] = recentError{
		Time:    time.Now(),
		Code:    code,
		Message: msg,
		Path:    path,
		Method:  method,
	}
	r.recentErrorsPtr = (r.recentErrorsPtr + 1) % recentErrorsLimit
	r.metricsMu.Unlock()
}

func (r *Router) GetMetrics() Metrics {
	r.metricsMu.Lock()
	defer r.metricsMu.Unlock()

	status := make(map[string]int, len(r.statusCounts))
	for k, v := range r.statusCounts {
		status[k] = v
	}

	recent := make([]recentError, 0, len(r.recentErrorsBuf))
	n := len(r.recentErrorsBuf)
	if n == recentErrorsLimit {
		for i := 0; i < n; i++ {
			idx := (r.recentErrorsPtr + i) % n
			recent = append(recent, r.recentErrorsBuf[idx])
		}
	} else {
		for i := 0; i < n; i++ {
			recent = append(recent, r.recentErrorsBuf[i])
		}
	}

	routeCount := 0
	for _, tree := range r.trees {
		routeCount += len(tree.Walk())
	}

	return Metrics{
		RouteCount:    routeCount,
		TotalRequests: atomic.LoadUint64(&r.totalRequests),
		StatusCounts:  status,
		RecentErrors:  recent,
		UptimeSeconds: time.Since(r.startedAt).Seconds(),
		StartedAt:     r.startedAt,
	}
}

func (r *Router) DumpRoutes() map[string]string {
	out := make(map[string]string)
	for method, tree := range r.trees {
		out[method] = tree.Dump()
	}
	return out
}

type RouteInfo struct {
	Method              string   `json:"method"`
	Path                string   `json:"path"`
	GlobalMwCount       int      `json:"global_mw_count"`
	GroupMwCount        int      `json:"group_mw_count"`
	TotalMwCount        int      `json:"total_mw_count"`
	HasHandler          bool     `json:"has_handler"`
	GlobalMwNames       []string `json:"global_mw_names,omitempty"`
	GroupMwNames        []string `json:"group_mw_names,omitempty"`
	HandlerName         string   `json:"handler_name,omitempty"`
}

func (r *Router) ListRoutes() []RouteInfo {
	var routes []RouteInfo
	globalNames := handlerNames(r.middleware)
	for method, tree := range r.trees {
		entries := tree.Walk()
		for _, e := range entries {
			entry, ok := e.Handler.(*routeEntry)
			if !ok {
				continue
			}
			routes = append(routes, RouteInfo{
				Method:        method,
				Path:          e.Path,
				GlobalMwCount: len(r.middleware),
				GroupMwCount:  len(entry.Middlewares),
				TotalMwCount:  len(r.middleware) + len(entry.Middlewares),
				HasHandler:    entry.Handler != nil,
				GlobalMwNames: append([]string{}, globalNames...),
				GroupMwNames:  handlerNames(entry.Middlewares),
				HandlerName:   funcName(entry.Handler),
			})
		}
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}
		return routes[i].Method < routes[j].Method
	})
	return routes
}

func (r *Router) PrintRoutes() string {
	routes := r.ListRoutes()
	if len(routes) == 0 {
		return "(no routes registered)"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Registered Routes (total: %d, global middleware: %d)\n", len(routes), len(r.middleware)))
	if len(r.middleware) > 0 {
		sb.WriteString(fmt.Sprintf("  Global: %s\n", strings.Join(handlerNames(r.middleware), " -> ")))
	}
	sb.WriteString(strings.Repeat("-", 110) + "\n")
	sb.WriteString(fmt.Sprintf("%-7s %-42s %-6s %-6s %s\n", "METHOD", "PATH", "GMW", "GRP", "HANDLER + MIDDLEWARE CHAIN (请求 → 响应的顺序)"))
	sb.WriteString(strings.Repeat("-", 110) + "\n")
	for _, rt := range routes {
		chain := make([]string, 0, rt.TotalMwCount+1)
		chain = append(chain, rt.GlobalMwNames...)
		chain = append(chain, rt.GroupMwNames...)
		chain = append(chain, "▶ "+rt.HandlerName)
		chainStr := strings.Join(chain, " → ")
		sb.WriteString(fmt.Sprintf("%-7s %-42s %-6d %-6d %s\n",
			rt.Method, rt.Path, rt.GlobalMwCount, rt.GroupMwCount, chainStr))
	}
	return sb.String()
}

type Group struct {
	prefix      string
	router      *Router
	middlewares []ctx.HandlerFunc
}

func (g *Group) Use(mw ...ctx.HandlerFunc) {
	g.middlewares = append(g.middlewares, mw...)
}

func (g *Group) GET(path string, handler ctx.HandlerFunc) {
	g.addRoute("GET", path, handler)
}

func (g *Group) POST(path string, handler ctx.HandlerFunc) {
	g.addRoute("POST", path, handler)
}

func (g *Group) PUT(path string, handler ctx.HandlerFunc) {
	g.addRoute("PUT", path, handler)
}

func (g *Group) DELETE(path string, handler ctx.HandlerFunc) {
	g.addRoute("DELETE", path, handler)
}

func (g *Group) PATCH(path string, handler ctx.HandlerFunc) {
	g.addRoute("PATCH", path, handler)
}

func (g *Group) HEAD(path string, handler ctx.HandlerFunc) {
	g.addRoute("HEAD", path, handler)
}

func (g *Group) OPTIONS(path string, handler ctx.HandlerFunc) {
	g.addRoute("OPTIONS", path, handler)
}

func (g *Group) Group(prefix string, handlers ...ctx.HandlerFunc) *Group {
	combined := make([]ctx.HandlerFunc, 0, len(g.middlewares)+len(handlers))
	combined = append(combined, g.middlewares...)
	combined = append(combined, handlers...)
	return &Group{
		prefix:      g.prefix + prefix,
		router:      g.router,
		middlewares: combined,
	}
}

func (g *Group) addRoute(method, path string, handler ctx.HandlerFunc) {
	fullPath := g.prefix + path
	g.router.addRoute(method, fullPath, handler, g.middlewares)
}

func Default() *Router {
	r := New()
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.RequestID())
	return r
}
