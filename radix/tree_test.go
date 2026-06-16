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

func TestStaticOverParamSameLevel(t *testing.T) {
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

func TestStaticFallbackToParam(t *testing.T) {
	tree := NewTree()
	staticCalled := false
	paramCalled := false

	tree.Add("GET", "/api/users/settings", func() { staticCalled = true })
	tree.Add("GET", "/api/:module/:action", func() { paramCalled = true })

	result := tree.Get("GET", "/api/users/settings")
	if !result.Found {
		t.Fatal("expected route to be found")
	}
	handler := result.Handler.(func())
	handler()
	if !staticCalled {
		t.Error("static route should match /api/users/settings")
	}
	if paramCalled {
		t.Error("param route should NOT match when static hits")
	}

	staticCalled = false
	paramCalled = false
	result = tree.Get("GET", "/api/users/profile")
	if !result.Found {
		t.Fatal("expected param route to match /api/users/profile via fallback")
	}
	handler = result.Handler.(func())
	handler()
	if !paramCalled {
		t.Error("param route should catch /api/users/profile when static doesn't match")
	}
	if staticCalled {
		t.Error("static route should NOT match /api/users/profile")
	}
}

func TestStaticFallbackToParamShorterDepth(t *testing.T) {
	tree := NewTree()
	settingsCalled := false
	moduleCalled := false

	tree.Add("GET", "/api/users/settings", func() { settingsCalled = true })
	tree.Add("GET", "/api/:module", func() { moduleCalled = true })

	result := tree.Get("GET", "/api/users")
	if !result.Found {
		t.Fatal("expected /api/:module to match /api/users")
	}
	handler := result.Handler.(func())
	handler()
	if !moduleCalled {
		t.Error("/api/:module should match /api/users via fallback")
	}
	if result.Params.ByName("module") != "users" {
		t.Errorf("expected module=users, got %s", result.Params.ByName("module"))
	}

	result = tree.Get("GET", "/api/users/settings")
	if !result.Found {
		t.Fatal("expected /api/users/settings to match static")
	}
	handler = result.Handler.(func())
	handler()
	if !settingsCalled {
		t.Error("static /api/users/settings should match exactly")
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

func TestWildcardLowestPriority(t *testing.T) {
	tree := NewTree()
	staticCalled := false
	wildCalled := false

	tree.Add("GET", "/files/special.txt", func() { staticCalled = true })
	tree.Add("GET", "/files/*rest", func() { wildCalled = true })

	result := tree.Get("GET", "/files/special.txt")
	if !result.Found {
		t.Fatal("expected route to be found")
	}
	handler := result.Handler.(func())
	handler()
	if !staticCalled {
		t.Error("static should win over wildcard for exact match")
	}
	if wildCalled {
		t.Error("wildcard should NOT be called when static matches")
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

func TestComplexBacktrack(t *testing.T) {
	tree := NewTree()
	deepStatic := false
	shallowParam := false

	tree.Add("GET", "/a/b/c/d/e", func() { deepStatic = true })
	tree.Add("GET", "/a/b/:x/:y", func() { shallowParam = true })

	result := tree.Get("GET", "/a/b/c/d/e")
	if !result.Found {
		t.Fatal("expected deep static to match")
	}
	result.Handler.(func())()
	if !deepStatic {
		t.Error("deep static should match exactly")
	}

	deepStatic = false
	result = tree.Get("GET", "/a/b/c/d")
	if !result.Found {
		t.Fatal("expected shallow param to match via fallback")
	}
	result.Handler.(func())()
	if !shallowParam {
		t.Error("param route should catch /a/b/c/d when deep static has no handler at that depth")
	}
	if result.Params.ByName("x") != "c" {
		t.Errorf("expected x=c, got %s", result.Params.ByName("x"))
	}
	if result.Params.ByName("y") != "d" {
		t.Errorf("expected y=d, got %s", result.Params.ByName("y"))
	}
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

func BenchmarkManySiblingRoutes_First(b *testing.B) {
	tree := NewTree()
	const N = 1000
	for i := 0; i < N; i++ {
		tree.Add("GET", fmt.Sprintf("/api/r%d", i), func() {})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Get("GET", "/api/r0")
	}
}

func BenchmarkManySiblingRoutes_Middle(b *testing.B) {
	tree := NewTree()
	const N = 1000
	for i := 0; i < N; i++ {
		tree.Add("GET", fmt.Sprintf("/api/r%d", i), func() {})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Get("GET", "/api/r500")
	}
}

func BenchmarkManySiblingRoutes_Last(b *testing.B) {
	tree := NewTree()
	const N = 1000
	for i := 0; i < N; i++ {
		tree.Add("GET", fmt.Sprintf("/api/r%d", i), func() {})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Get("GET", "/api/r999")
	}
}

func BenchmarkManyDeepRoutes(b *testing.B) {
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

func BenchmarkTenThousandRoutes(b *testing.B) {
	tree := NewTree()
	const N = 10000
	for i := 0; i < N; i++ {
		tree.Add("GET", fmt.Sprintf("/resources/type-%d/items/%d", i%100, i), func() {})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Get("GET", "/resources/type-50/items/5000")
	}
}
