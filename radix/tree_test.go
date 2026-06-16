package radix

import (
	"fmt"
	"testing"
)

func TestStaticRoute(t *testing.T) {
	tree := NewTree()
	called := false
	tree.Add("GET", "/hello", func() { called = true })

	result := tree.Get("GET", "/hello")
	if !result.Found {
		t.Fatal("expected route to be found")
	}
	h, ok := result.Handler.(func())
	if !ok || h == nil {
		t.Fatal("expected handler to be set")
	}
	h()
	if !called {
		t.Fatal("expected handler to be the correct one")
	}
}

func TestParamRoute(t *testing.T) {
	tree := NewTree()
	tree.Add("GET", "/users/:id", func() {})

	result := tree.Get("GET", "/users/42")
	if !result.Found {
		t.Fatal("expected route to be found")
	}
	if result.Params.ByName("id") != "42" {
		t.Fatalf("expected param id=42, got %s", result.Params.ByName("id"))
	}
}

func TestWildcardRoute(t *testing.T) {
	tree := NewTree()
	tree.Add("GET", "/files/*filepath", func() {})

	result := tree.Get("GET", "/files/docs/readme.md")
	if !result.Found {
		t.Fatal("expected route to be found")
	}
	if result.Params.ByName("filepath") != "docs/readme.md" {
		t.Fatalf("expected param filepath=docs/readme.md, got %s", result.Params.ByName("filepath"))
	}
}

func TestStaticOverParam(t *testing.T) {
	tree := NewTree()
	staticCalled := false
	paramCalled := false

	tree.Add("GET", "/users/me", func() { staticCalled = true })
	tree.Add("GET", "/users/:id", func() { paramCalled = true })

	result := tree.Get("GET", "/users/me")
	if !result.Found {
		t.Fatal("expected route to be found")
	}
	handler := result.Handler.(func())
	handler()
	if !staticCalled {
		t.Error("static route should match /users/me")
	}
	if paramCalled {
		t.Error("param route should NOT match /users/me when static exists")
	}

	paramCalled = false
	result = tree.Get("GET", "/users/123")
	if !result.Found {
		t.Fatal("expected param route to be found")
	}
	handler = result.Handler.(func())
	handler()
	if !paramCalled {
		t.Error("param route should match /users/123")
	}
}

func TestNotFound(t *testing.T) {
	tree := NewTree()
	tree.Add("GET", "/hello", func() {})

	result := tree.Get("GET", "/world")
	if result.Found {
		t.Fatal("expected route NOT to be found")
	}
}

func TestMultipleParams(t *testing.T) {
	tree := NewTree()
	tree.Add("GET", "/users/:uid/posts/:pid", func() {})

	result := tree.Get("GET", "/users/5/posts/99")
	if !result.Found {
		t.Fatal("expected route to be found")
	}
	if result.Params.ByName("uid") != "5" {
		t.Fatalf("expected uid=5, got %s", result.Params.ByName("uid"))
	}
	if result.Params.ByName("pid") != "99" {
		t.Fatalf("expected pid=99, got %s", result.Params.ByName("pid"))
	}
}

func TestRootRoute(t *testing.T) {
	tree := NewTree()
	called := false
	tree.Add("GET", "/", func() { called = true })

	result := tree.Get("GET", "/")
	if !result.Found {
		t.Fatal("expected root route to be found")
	}
	handler := result.Handler.(func())
	handler()
	if !called {
		t.Fatal("expected handler to be called")
	}
}

func TestPriorityStaticBeforeParam(t *testing.T) {
	tree := NewTree()
	tree.Add("GET", "/api/users", func() {})
	tree.Add("GET", "/api/:module", func() {})

	result := tree.Get("GET", "/api/users")
	if !result.Found {
		t.Fatal("expected route to be found")
	}

	result = tree.Get("GET", "/api/settings")
	if !result.Found {
		t.Fatal("expected param route to be found for /api/settings")
	}
}

func TestWildcardCatchesAll(t *testing.T) {
	tree := NewTree()
	tree.Add("GET", "/assets/*path", func() {})

	cases := []string{
		"/assets/style.css",
		"/assets/js/app.js",
		"/assets/img/logo/png/logo.png",
	}

	for _, path := range cases {
		result := tree.Get("GET", path)
		if !result.Found {
			t.Fatalf("expected wildcard to match %s", path)
		}
	}
}

func TestRouteConflict(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on route conflict")
		}
	}()

	tree := NewTree()
	tree.Add("GET", "/hello", func() {})
	tree.Add("GET", "/hello", func() {})
}

func BenchmarkStaticRoute(b *testing.B) {
	tree := NewTree()
	tree.Add("GET", "/hello", func() {})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Get("GET", "/hello")
	}
}

func BenchmarkParamRoute(b *testing.B) {
	tree := NewTree()
	tree.Add("GET", "/users/:id", func() {})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Get("GET", "/users/42")
	}
}

func BenchmarkManyRoutes(b *testing.B) {
	tree := NewTree()
	for i := 0; i < 1000; i++ {
		tree.Add("GET", fmt.Sprintf("/api/v1/r%d/item%d", i, i), func() {})
	}
	tree.Add("GET", "/users/:id/posts/:pid", func() {})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Get("GET", "/users/42/posts/99")
	}
}
