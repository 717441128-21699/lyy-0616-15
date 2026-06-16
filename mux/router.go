package mux

import (
	"net/http"

	"github.com/trae-framework/vine/ctx"
	"github.com/trae-framework/vine/middleware"
	"github.com/trae-framework/vine/radix"
)

type Router struct {
	trees      map[string]*radix.Tree
	middleware []ctx.HandlerFunc
	notFound   ctx.HandlerFunc
}

func New() *Router {
	r := &Router{
		trees: make(map[string]*radix.Tree),
		notFound: func(c *ctx.Ctx) {
			c.JSON(http.StatusNotFound, map[string]string{
				"error": "404 page not found",
			})
		},
	}
	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"} {
		r.trees[method] = radix.NewTree()
	}
	return r
}

func (r *Router) Use(mw ...ctx.HandlerFunc) {
	r.middleware = append(r.middleware, mw...)
}

func (r *Router) GET(path string, handler ctx.HandlerFunc) {
	r.addRoute("GET", path, handler)
}

func (r *Router) POST(path string, handler ctx.HandlerFunc) {
	r.addRoute("POST", path, handler)
}

func (r *Router) PUT(path string, handler ctx.HandlerFunc) {
	r.addRoute("PUT", path, handler)
}

func (r *Router) DELETE(path string, handler ctx.HandlerFunc) {
	r.addRoute("DELETE", path, handler)
}

func (r *Router) PATCH(path string, handler ctx.HandlerFunc) {
	r.addRoute("PATCH", path, handler)
}

func (r *Router) Group(prefix string, handlers ...ctx.HandlerFunc) *Group {
	return &Group{
		prefix:      prefix,
		router:      r,
		middlewares: append([]ctx.HandlerFunc{}, handlers...),
	}
}

func (r *Router) addRoute(method, path string, handler ctx.HandlerFunc) {
	tree, ok := r.trees[method]
	if !ok {
		tree = radix.NewTree()
		r.trees[method] = tree
	}
	tree.Add(method, path, handler)
}

func (r *Router) SetNotFound(handler ctx.HandlerFunc) {
	r.notFound = handler
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c := ctx.New(w, req)
	defer c.Release()

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

	handler, ok := result.Handler.(ctx.HandlerFunc)
	if !ok {
		r.notFound(c)
		return
	}

	c.SetParams(result.Params)

	handlers := make([]ctx.HandlerFunc, 0, len(r.middleware)+1)
	handlers = append(handlers, r.middleware...)
	handlers = append(handlers, handler)

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

	if len(g.middlewares) == 0 {
		g.router.addRoute(method, fullPath, handler)
		return
	}

	mws := make([]ctx.HandlerFunc, len(g.middlewares))
	copy(mws, g.middlewares)

	originalHandler := handler
	wrappedHandler := func(c *ctx.Ctx) {
		allHandlers := make([]ctx.HandlerFunc, 0, len(mws)+1)
		allHandlers = append(allHandlers, mws...)
		allHandlers = append(allHandlers, originalHandler)

		c.SetHandlers(allHandlers)
		c.Next()
	}

	g.router.addRoute(method, fullPath, wrappedHandler)
}

func Default() *Router {
	r := New()
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.RequestID())
	return r
}
