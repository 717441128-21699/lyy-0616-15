package mux

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trae-framework/vine/ctx"
	"github.com/trae-framework/vine/middleware"
)

func TestBasicRouting(t *testing.T) {
	r := New()
	r.GET("/hello", func(c *ctx.Ctx) {
		c.JSON(http.StatusOK, map[string]string{"message": "hello"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/hello", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestParamRouting(t *testing.T) {
	r := New()
	r.GET("/users/:id", func(c *ctx.Ctx) {
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/users/42", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["id"] != "42" {
		t.Fatalf("expected id=42, got %s", body["id"])
	}
}

func TestWildcardRouting(t *testing.T) {
	r := New()
	r.GET("/files/*filepath", func(c *ctx.Ctx) {
		c.JSON(http.StatusOK, map[string]string{"filepath": c.Param("filepath")})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/files/docs/readme.md", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["filepath"] != "docs/readme.md" {
		t.Fatalf("expected filepath=docs/readme.md, got %s", body["filepath"])
	}
}

func TestNotFound(t *testing.T) {
	r := New()
	r.GET("/exists", func(c *ctx.Ctx) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notexists", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestMiddleware(t *testing.T) {
	r := New()

	order := []int{}
	r.Use(func(c *ctx.Ctx) {
		order = append(order, 1)
		c.Next()
		order = append(order, 4)
	})
	r.Use(func(c *ctx.Ctx) {
		order = append(order, 2)
		c.Next()
		order = append(order, 3)
	})
	r.GET("/test", func(c *ctx.Ctx) {
		order = append(order, 100)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	expected := []int{1, 2, 100, 3, 4}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("onion model: expected order[%d]=%d, got %d, full=%v", i, v, order[i], order)
		}
	}
}

func TestPanicRecovery(t *testing.T) {
	r := New()
	r.Use(middleware.Recovery())
	r.GET("/panic", func(c *ctx.Ctx) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panic", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 after panic recovery, got %d", w.Code)
	}
}

func TestGroupMiddleware(t *testing.T) {
	r := New()
	order := []int{}

	api := r.Group("/api")
	api.Use(func(c *ctx.Ctx) {
		order = append(order, 1)
		c.Next()
		order = append(order, 2)
	})
	api.GET("/hello", func(c *ctx.Ctx) {
		order = append(order, 100)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/hello", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	expected := []int{1, 100, 2}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("group middleware: expected order[%d]=%d, got %d, full=%v", i, v, order[i], order)
		}
	}
}

func TestContextPassData(t *testing.T) {
	r := New()
	r.Use(func(c *ctx.Ctx) {
		c.Set("user_id", "42")
		c.Next()
	})
	r.GET("/me", func(c *ctx.Ctx) {
		userID := c.GetString("user_id")
		c.JSON(http.StatusOK, map[string]string{"user_id": userID})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/me", nil)
	r.ServeHTTP(w, req)

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["user_id"] != "42" {
		t.Fatalf("expected user_id=42, got %s", body["user_id"])
	}
}

func TestStaticOverParamPriority(t *testing.T) {
	r := New()
	staticCalled := false
	paramCalled := false

	r.GET("/users/me", func(c *ctx.Ctx) {
		staticCalled = true
		c.String(http.StatusOK, "static")
	})
	r.GET("/users/:id", func(c *ctx.Ctx) {
		paramCalled = true
		c.String(http.StatusOK, "param")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/users/me", nil)
	r.ServeHTTP(w, req)

	if !staticCalled {
		t.Error("static route /users/me should match first")
	}

	staticCalled = false
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/users/123", nil)
	r.ServeHTTP(w, req)

	if !paramCalled {
		t.Error("param route /users/:id should match /users/123")
	}
}

func TestDifferentMethods(t *testing.T) {
	r := New()

	r.GET("/resource", func(c *ctx.Ctx) {
		c.String(http.StatusOK, "GET")
	})
	r.POST("/resource", func(c *ctx.Ctx) {
		c.String(http.StatusOK, "POST")
	})
	r.PUT("/resource", func(c *ctx.Ctx) {
		c.String(http.StatusOK, "PUT")
	})
	r.DELETE("/resource", func(c *ctx.Ctx) {
		c.String(http.StatusOK, "DELETE")
	})

	methods := []string{"GET", "POST", "PUT", "DELETE"}
	for _, method := range methods {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/resource", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d", method, w.Code)
		}
	}
}

func TestGlobalAndGroupMiddleware_OnionOrder(t *testing.T) {
	r := New()
	order := []int{}

	r.Use(func(c *ctx.Ctx) {
		order = append(order, 1)
		c.Next()
		order = append(order, 8)
	})
	r.Use(func(c *ctx.Ctx) {
		order = append(order, 2)
		c.Next()
		order = append(order, 7)
	})

	api := r.Group("/api", func(c *ctx.Ctx) {
		order = append(order, 3)
		c.Next()
		order = append(order, 6)
	})
	api.Use(func(c *ctx.Ctx) {
		order = append(order, 4)
		c.Next()
		order = append(order, 5)
	})
	api.GET("/test", func(c *ctx.Ctx) {
		order = append(order, 100)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/test", nil)
	r.ServeHTTP(w, req)

	expected := []int{1, 2, 3, 4, 100, 5, 6, 7, 8}
	if len(order) != len(expected) {
		t.Fatalf("expected %d steps, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("onion order mismatch at %d: expected %d, got %d\n  full order: %v\n  expected:    %v",
				i, v, order[i], order, expected)
		}
	}
}

func TestNestedGroups_ThreeLevels(t *testing.T) {
	r := New()
	order := []int{}

	r.Use(func(c *ctx.Ctx) {
		order = append(order, 1)
		c.Next()
		order = append(order, 10)
	})

	api := r.Group("/api", func(c *ctx.Ctx) {
		order = append(order, 2)
		c.Next()
		order = append(order, 9)
	})

	admin := api.Group("/admin", func(c *ctx.Ctx) {
		order = append(order, 3)
		c.Next()
		order = append(order, 8)
	})

	dashboard := admin.Group("/dashboard", func(c *ctx.Ctx) {
		order = append(order, 4)
		c.Next()
		order = append(order, 7)
	})

	dashboard.GET("/stats", func(c *ctx.Ctx) {
		order = append(order, 100)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/dashboard/stats", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	expected := []int{1, 2, 3, 4, 100, 7, 8, 9, 10}
	if len(order) != len(expected) {
		t.Fatalf("expected %d steps, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("nested onion order mismatch at %d: expected %d, got %d\n  got: %v\n  want: %v",
				i, v, order[i], order, expected)
		}
	}
}

func TestNestedGroupWithDifferentDepths(t *testing.T) {
	r := New()
	order := []int{}

	r.Use(func(c *ctx.Ctx) {
		order = append(order, 1)
		c.Next()
		order = append(order, 6)
	})

	api := r.Group("/api", func(c *ctx.Ctx) {
		order = append(order, 2)
		c.Next()
		order = append(order, 5)
	})

	api.GET("/shallow", func(c *ctx.Ctx) {
		order = append(order, 10)
		c.String(http.StatusOK, "shallow")
	})

	deep := api.Group("/deep", func(c *ctx.Ctx) {
		order = append(order, 3)
		c.Next()
		order = append(order, 4)
	})
	deep.GET("/route", func(c *ctx.Ctx) {
		order = append(order, 20)
		c.String(http.StatusOK, "deep")
	})

	t.Run("shallow_route", func(t *testing.T) {
		order = []int{}
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/shallow", nil)
		r.ServeHTTP(w, req)

		expected := []int{1, 2, 10, 5, 6}
		if len(order) != len(expected) {
			t.Fatalf("shallow: expected %d steps, got %d: %v", len(expected), len(order), order)
		}
		for i, v := range expected {
			if order[i] != v {
				t.Fatalf("shallow: order[%d]=%d, want %d; full=%v", i, order[i], v, order)
			}
		}
	})

	t.Run("deep_route", func(t *testing.T) {
		order = []int{}
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/deep/route", nil)
		r.ServeHTTP(w, req)

		expected := []int{1, 2, 3, 20, 4, 5, 6}
		if len(order) != len(expected) {
			t.Fatalf("deep: expected %d steps, got %d: %v", len(expected), len(order), order)
		}
		for i, v := range expected {
			if order[i] != v {
				t.Fatalf("deep: order[%d]=%d, want %d; full=%v", i, order[i], v, order)
			}
		}
	})
}
