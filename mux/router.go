package mux

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

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

type Router struct {
	trees        map[string]*radix.Tree
	middleware   []ctx.HandlerFunc
	notFound     ctx.HandlerFunc
	errorHandler ErrorHandlerFunc
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
		errorHandler: defaultErrorHandler,
	}
	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"} {
		r.trees[method] = radix.NewTree()
	}
	return r
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
	c := ctx.New(w, req)
	defer c.Release()

	c.SetErrorHandler(func(c *ctx.Ctx, err error) {
		r.errorHandler(c, err)
	})

	tree, ok := r.trees[req.Method]
	if !ok {
		r.notFound(c)
		return
	}

	result := tree.Get(req.Method, req.URL.Path)
	if !result.Found {
		if result.TSR {
			c.JSON(http.StatusMovedPermanently, map[string]string{
				"redirect": result.TSRPath,
			})
			return
		}
		r.notFound(c)
		return
	}

	entry, ok := result.Handler.(*routeEntry)
	if !ok {
		r.notFound(c)
		return
	}

	c.SetParams(result.Params)

	handlers := make([]ctx.HandlerFunc, 0, len(r.middleware)+len(entry.Middlewares)+1)
	handlers = append(handlers, r.middleware...)
	handlers = append(handlers, entry.Middlewares...)
	handlers = append(handlers, entry.Handler)

	c.SetHandlers(handlers)
	c.Next()
}

func (r *Router) DumpRoutes() map[string]string {
	out := make(map[string]string)
	for method, tree := range r.trees {
		out[method] = tree.Dump()
	}
	return out
}

type RouteInfo struct {
	Method           string
	Path             string
	GlobalMwCount    int
	GroupMwCount     int
	TotalMwCount     int
	HasHandler       bool
}

func (r *Router) ListRoutes() []RouteInfo {
	var routes []RouteInfo
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
	sb.WriteString(strings.Repeat("-", 90) + "\n")
	sb.WriteString(fmt.Sprintf("%-7s %-40s %-8s %-8s %s\n", "METHOD", "PATH", "GLOBAL", "GROUP", "TOTAL"))
	sb.WriteString(strings.Repeat("-", 90) + "\n")
	for _, rt := range routes {
		sb.WriteString(fmt.Sprintf("%-7s %-40s %-8d %-8d %d\n",
			rt.Method, rt.Path, rt.GlobalMwCount, rt.GroupMwCount, rt.TotalMwCount))
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
