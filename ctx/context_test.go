package ctx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trae-framework/vine/radix"
)

func TestContextSetGet(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	c := New(w, r)

	c.Set("key", "value")
	val, ok := c.Get("key")
	if !ok || val != "value" {
		t.Fatalf("expected value, got %v, ok=%v", val, ok)
	}
}

func TestContextParam(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/users/42", nil)
	c := New(w, r)

	c.SetParams(radix.Params{{Key: "id", Value: "42"}})
	if c.Param("id") != "42" {
		t.Fatalf("expected id=42, got %s", c.Param("id"))
	}
}

func TestContextQuery(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test?page=3&limit=20", nil)
	c := New(w, r)

	if c.Query("page") != "3" {
		t.Fatalf("expected page=3, got %s", c.Query("page"))
	}
	if c.QueryInt("page", 1) != 3 {
		t.Fatalf("expected page=3, got %d", c.QueryInt("page", 1))
	}
	if c.QueryInt("missing", 99) != 99 {
		t.Fatalf("expected default 99, got %d", c.QueryInt("missing", 99))
	}
}

func TestContextJSON(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := New(w, r)

	c.JSON(http.StatusOK, map[string]string{"hello": "world"})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Fatalf("expected application/json, got %s", ct)
	}
}

func TestContextNext(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := New(w, r)

	order := []int{}

	mw1 := func(c *Ctx) {
		order = append(order, 1)
		c.Next()
		order = append(order, 4)
	}
	mw2 := func(c *Ctx) {
		order = append(order, 2)
		c.Next()
		order = append(order, 3)
	}
	handler := func(c *Ctx) {
		order = append(order, 100)
	}

	c.SetHandlers([]HandlerFunc{mw1, mw2, handler})
	c.Next()

	expected := []int{1, 2, 100, 3, 4}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("onion model failed: expected order[%d]=%d, got %d, full order=%v", i, v, order[i], order)
		}
	}
}

func TestContextAbort(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := New(w, r)

	called := false
	mw := func(c *Ctx) {
		c.Abort()
	}
	handler := func(c *Ctx) {
		called = true
	}

	c.SetHandlers([]HandlerFunc{mw, handler})
	c.Next()

	if called {
		t.Fatal("handler should NOT be called after abort")
	}
	if !c.IsAborted() {
		t.Fatal("context should be aborted")
	}
}

func TestContextPanicRecovery(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	c := New(w, r)

	recovery := func(c *Ctx) {
		defer func() {
			if err := recover(); err != nil {
				c.JSON(http.StatusInternalServerError, map[string]string{"error": "recovered"})
			}
		}()
		c.Next()
	}

	panicHandler := func(c *Ctx) {
		panic("oops!")
	}

	c.SetHandlers([]HandlerFunc{recovery, panicHandler})
	c.Next()

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestContextBindAndValidate(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name" form:"name" required:"true" min:"2" max:"50"`
		Age  int    `json:"age"  form:"age"  required:"true" min:"1" max:"150"`
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("Content-Type", "application/json")
	c := New(w, r)

	var obj TestStruct
	obj.Name = "Alice"
	obj.Age = 25

	if err := c.Validate(&obj); err != nil {
		t.Fatalf("expected validation to pass, got %v", err)
	}
}

func TestContextClientIP(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	c := New(w, r)

	if ip := c.ClientIP(); ip != "1.2.3.4" {
		t.Fatalf("expected 1.2.3.4, got %s", ip)
	}
}
