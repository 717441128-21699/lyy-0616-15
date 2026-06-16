package mux

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trae-framework/vine/ctx"
	"github.com/trae-framework/vine/errors"
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

type testErrResp struct {
	Success bool        `json:"success"`
	Error   testErrBody `json:"error"`
}

type testErrBody struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Detail  interface{} `json:"detail,omitempty"`
}

func decodeTestErr(t *testing.T, body string) testErrResp {
	t.Helper()
	var resp testErrResp
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("failed to decode error response: %v\nbody: %s", err, body)
	}
	return resp
}

func TestGlobalErrorHandler_Custom(t *testing.T) {
	r := New()

	r.SetErrorHandler(func(c *ctx.Ctx, err error) {
		appErr := errors.FromError(err)
		resp := testErrResp{
			Success: false,
			Error: testErrBody{
				Code:    appErr.Code,
				Message: appErr.Message,
				Detail:  appErr.Detail,
			},
		}
		c.JSON(appErr.Code, resp)
	})

	r.GET("/bad", func(c *ctx.Ctx) {
		c.BadRequest("param x is required")
	})
	r.GET("/notfound", func(c *ctx.Ctx) {
		c.NotFound("user 999")
	})
	r.GET("/forbidden", func(c *ctx.Ctx) {
		c.Forbidden("admin only")
	})
	r.GET("/plain", func(c *ctx.Ctx) {
		c.HandleError(fmt.Errorf("database down"))
	})

	t.Run("bad_request_400", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/bad", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", w.Code)
		}
		resp := decodeTestErr(t, w.Body.String())
		if resp.Success {
			t.Fatal("success should be false")
		}
		if resp.Error.Code != http.StatusBadRequest {
			t.Fatalf("want code %d, got %d", http.StatusBadRequest, resp.Error.Code)
		}
		if resp.Error.Message != "param x is required" {
			t.Fatalf("unexpected message: %s", resp.Error.Message)
		}
	})

	t.Run("not_found_404", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/notfound", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("want 404, got %d", w.Code)
		}
		resp := decodeTestErr(t, w.Body.String())
		if resp.Error.Code != http.StatusNotFound {
			t.Fatalf("want code %d, got %d", http.StatusNotFound, resp.Error.Code)
		}
		if resp.Error.Message != "user 999" {
			t.Fatalf("unexpected message: %s", resp.Error.Message)
		}
	})

	t.Run("forbidden_403", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/forbidden", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("want 403, got %d", w.Code)
		}
		resp := decodeTestErr(t, w.Body.String())
		if resp.Error.Code != http.StatusForbidden {
			t.Fatalf("want code %d, got %d", http.StatusForbidden, resp.Error.Code)
		}
	})

	t.Run("plain_error_becomes_500", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/plain", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("want 500, got %d", w.Code)
		}
		resp := decodeTestErr(t, w.Body.String())
		if resp.Error.Code != http.StatusInternalServerError {
			t.Fatalf("want code %d, got %d", http.StatusInternalServerError, resp.Error.Code)
		}
		if resp.Error.Message != "database down" {
			t.Fatalf("unexpected message: %s", resp.Error.Message)
		}
	})
}

func TestGlobalErrorHandler_WithRecovery(t *testing.T) {
	r := New()
	r.Use(middleware.Recovery())

	r.SetErrorHandler(func(c *ctx.Ctx, err error) {
		appErr := errors.FromError(err)
		resp := testErrResp{
			Success: false,
			Error: testErrBody{
				Code:    appErr.Code,
				Message: appErr.Message,
			},
		}
		c.JSON(appErr.Code, resp)
	})

	r.GET("/panic", func(c *ctx.Ctx) {
		panic("boom! critical failure")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panic", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 after panic, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("want JSON content-type, got %s", ct)
	}
	resp := decodeTestErr(t, w.Body.String())
	if resp.Success {
		t.Fatal("success should be false after panic")
	}
	if resp.Error.Code != http.StatusInternalServerError {
		t.Fatalf("want code 500, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "boom!") {
		t.Fatalf("panic message should be preserved, got: %s", resp.Error.Message)
	}
}

func TestDifferentMethods_Errors(t *testing.T) {
	r := New()
	r.SetErrorHandler(func(c *ctx.Ctx, err error) {
		appErr := errors.FromError(err)
		c.JSON(appErr.Code, appErr)
	})

	r.GET("/resource/:id", func(c *ctx.Ctx) {
		c.NotFoundf("resource %s not found", c.Param("id"))
	})
	r.POST("/resource", func(c *ctx.Ctx) {
		c.BadRequest("invalid JSON body")
	})
	r.PUT("/resource/:id", func(c *ctx.Ctx) {
		c.ValidationError("field 'name' is required")
	})
	r.DELETE("/resource/:id", func(c *ctx.Ctx) {
		c.Conflict("resource is in use, cannot delete")
	})

	t.Run("GET_404", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/resource/42", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("GET: want 404, got %d", w.Code)
		}
	})

	t.Run("POST_400", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/resource", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("POST: want 400, got %d", w.Code)
		}
	})

	t.Run("PUT_422", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/resource/1", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("PUT: want 422, got %d", w.Code)
		}
	})

	t.Run("DELETE_409", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("DELETE", "/resource/1", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("DELETE: want 409, got %d", w.Code)
		}
	})
}

func TestOptionsAndHeadRequests(t *testing.T) {
	r := New()
	r.Use(middleware.CORS("*"))

	r.GET("/data", func(c *ctx.Ctx) {
		c.JSON(http.StatusOK, map[string]string{"hello": "world"})
	})
	r.HEAD("/data", func(c *ctx.Ctx) {
		c.Status(http.StatusOK)
		c.Header("Content-Length", "18")
	})
	r.OPTIONS("/data", func(c *ctx.Ctx) {
		c.Status(http.StatusNoContent)
	})

	t.Run("OPTIONS_returns_204", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("OPTIONS", "/data", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("OPTIONS: want 204, got %d", w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatal("missing CORS origin header")
		}
	})

	t.Run("HEAD_no_body", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("HEAD", "/data", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("HEAD: want 200, got %d", w.Code)
		}
		if w.Body.Len() != 0 {
			t.Fatalf("HEAD should have empty body, got %d bytes", w.Body.Len())
		}
	})
}

func TestAuthMiddleware_Distinguishes401And403(t *testing.T) {
	r := New()
	r.SetErrorHandler(func(c *ctx.Ctx, err error) {
		appErr := errors.FromError(err)
		c.JSON(appErr.Code, appErr)
	})

	api := r.Group("/api")
	api.Use(middleware.Auth(""))

	admin := api.Group("/admin", middleware.Auth("admin"))
	admin.GET("/dashboard", func(c *ctx.Ctx) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	api.GET("/profile", func(c *ctx.Ctx) {
		c.JSON(http.StatusOK, map[string]string{"status": "profile"})
	})

	t.Run("no_token_401", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/profile", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("no token: want 401, got %d", w.Code)
		}
		var resp errors.AppError
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("body code should be 401, got %d", resp.Code)
		}
	})

	t.Run("invalid_format_401", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/profile", nil)
		req.Header.Set("Authorization", "Token xxx")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("invalid format: want 401, got %d", w.Code)
		}
	})

	t.Run("user_token_profile_200", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/profile", nil)
		req.Header.Set("Authorization", "Bearer alice")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("user profile: want 200, got %d", w.Code)
		}
	})

	t.Run("user_token_admin_403", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/admin/dashboard", nil)
		req.Header.Set("Authorization", "Bearer alice")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("user on admin: want 403, got %d", w.Code)
		}
		var resp errors.AppError
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if resp.Code != http.StatusForbidden {
			t.Fatalf("body code should be 403, got %d", resp.Code)
		}
	})

	t.Run("admin_token_admin_200", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/admin/dashboard", nil)
		req.Header.Set("Authorization", "Bearer admin")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("admin on admin: want 200, got %d", w.Code)
		}
	})
}

func TestListRoutes_And_PrintRoutes(t *testing.T) {
	r := New()
	r.Use(middleware.Logger())
	r.Use(middleware.RequestID())

	r.GET("/", func(c *ctx.Ctx) {})
	r.GET("/health", func(c *ctx.Ctx) {})

	api := r.Group("/api")
	api.Use(middleware.Auth("user"))
	api.GET("/profile", func(c *ctx.Ctx) {})
	api.POST("/orders", func(c *ctx.Ctx) {})

	admin := api.Group("/admin")
	admin.Use(middleware.Auth("admin"))
	admin.GET("/dashboard", func(c *ctx.Ctx) {})

	t.Run("ListRoutes_count_and_grouping", func(t *testing.T) {
		routes := r.ListRoutes()
		if len(routes) != 5 {
			t.Fatalf("want 5 routes, got %d", len(routes))
		}

		m := make(map[string]RouteInfo)
		for _, rt := range routes {
			key := rt.Method + " " + rt.Path
			m[key] = rt
		}

		if _, ok := m["GET /"]; !ok {
			t.Fatal("missing GET /")
		}
		if rt, ok := m["GET /api/profile"]; !ok {
			t.Fatal("missing GET /api/profile")
		} else {
			if rt.GlobalMwCount != 2 {
				t.Fatalf("GET /api/profile should have 2 global MW, got %d", rt.GlobalMwCount)
			}
			if rt.GroupMwCount < 1 {
				t.Fatalf("GET /api/profile should have >=1 group MW, got %d", rt.GroupMwCount)
			}
		}
		if rt, ok := m["GET /api/admin/dashboard"]; !ok {
			t.Fatal("missing GET /api/admin/dashboard")
		} else {
			if rt.GroupMwCount < 2 {
				t.Fatalf("nested admin route should have >=2 group MW, got %d", rt.GroupMwCount)
			}
		}
	})

	t.Run("PrintRoutes_not_empty", func(t *testing.T) {
		out := r.PrintRoutes()
		if out == "" || out == "(no routes registered)" {
			t.Fatal("PrintRoutes should return non-empty table")
		}
		if !strings.Contains(out, "METHOD") || !strings.Contains(out, "PATH") {
			t.Fatalf("PrintRoutes should have table header, got:\n%s", out)
		}
		if !strings.Contains(out, "/api/admin/dashboard") {
			t.Fatalf("PrintRoutes should contain nested route, got:\n%s", out)
		}
	})
}

func TestListRoutes_WithMiddlewareNames(t *testing.T) {
	r := New()
	r.Use(middleware.Logger())
	r.Use(middleware.RequestID())

	api := r.Group("/api", middleware.Auth(""))
	api.GET("/hello", func(c *ctx.Ctx) {})

	admin := api.Group("/admin", middleware.Auth("admin"))
	admin.GET("/dashboard", func(c *ctx.Ctx) {})

	routes := r.ListRoutes()
	if len(routes) != 2 {
		t.Fatalf("want 2 routes, got %d", len(routes))
	}

	t.Run("global_mw_names", func(t *testing.T) {
		for _, rt := range routes {
			if len(rt.GlobalMwNames) != 2 {
				t.Fatalf("route %s %s: want 2 global mw names, got %d (%v)",
					rt.Method, rt.Path, len(rt.GlobalMwNames), rt.GlobalMwNames)
			}
			if !strings.Contains(rt.GlobalMwNames[0], "Logger") &&
				!strings.Contains(rt.GlobalMwNames[0], "RequestID") {
				t.Fatalf("first global mw name should be recognizable, got: %v", rt.GlobalMwNames)
			}
		}
	})

	t.Run("group_mw_names_nested", func(t *testing.T) {
		m := make(map[string]RouteInfo)
		for _, rt := range routes {
			m[rt.Method+" "+rt.Path] = rt
		}

		apiRoute := m["GET /api/hello"]
		if len(apiRoute.GroupMwNames) != 1 {
			t.Fatalf("api/hello should have 1 group mw, got %d: %v",
				len(apiRoute.GroupMwNames), apiRoute.GroupMwNames)
		}

		adminRoute := m["GET /api/admin/dashboard"]
		if len(adminRoute.GroupMwNames) != 2 {
			t.Fatalf("admin/dashboard should have 2 group mw, got %d: %v",
				len(adminRoute.GroupMwNames), adminRoute.GroupMwNames)
		}
		if !strings.Contains(adminRoute.GroupMwNames[0], "Auth") ||
			!strings.Contains(adminRoute.GroupMwNames[1], "Auth") {
			t.Fatalf("nested admin route should show 2 Auth mws, got: %v", adminRoute.GroupMwNames)
		}
	})

	t.Run("PrintRoutes_shows_chain", func(t *testing.T) {
		out := r.PrintRoutes()
		if !strings.Contains(out, "→") {
			t.Fatalf("PrintRoutes should show middleware chain with arrows, got:\n%s", out)
		}
		if !strings.Contains(out, "▶") {
			t.Fatalf("PrintRoutes should show handler marker, got:\n%s", out)
		}
	})
}

func TestMetrics_RequestCountAndStatus(t *testing.T) {
	r := New()

	r.GET("/ok", func(c *ctx.Ctx) {
		c.String(http.StatusOK, "ok")
	})
	r.GET("/err400", func(c *ctx.Ctx) {
		c.BadRequest("bad")
	})
	r.GET("/err404", func(c *ctx.Ctx) {
		c.NotFound("gone")
	})

	m0 := r.GetMetrics()
	if m0.TotalRequests != 0 {
		t.Fatalf("initial total should be 0, got %d", m0.TotalRequests)
	}

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/ok", nil)
		r.ServeHTTP(w, req)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/err400", nil))
	r.ServeHTTP(w, httptest.NewRequest("GET", "/err404", nil))

	m1 := r.GetMetrics()
	if m1.TotalRequests != 5 {
		t.Fatalf("want total=5, got %d", m1.TotalRequests)
	}
	if m1.StatusCounts["200"] != 3 {
		t.Fatalf("want 3x200, got %d", m1.StatusCounts["200"])
	}
	if m1.StatusCounts["400"] != 1 {
		t.Fatalf("want 1x400, got %d", m1.StatusCounts["400"])
	}
	if m1.StatusCounts["404"] != 1 {
		t.Fatalf("want 1x404, got %d", m1.StatusCounts["404"])
	}
	if m1.RouteCount != 3 {
		t.Fatalf("route count should be 3, got %d", m1.RouteCount)
	}
	if m1.UptimeSeconds < 0 {
		t.Fatalf("uptime should be >= 0, got %v", m1.UptimeSeconds)
	}
}

func TestMetrics_RecentErrors(t *testing.T) {
	r := New()

	r.GET("/err-a", func(c *ctx.Ctx) {
		c.BadRequest("error A")
	})
	r.GET("/err-b", func(c *ctx.Ctx) {
		c.Forbidden("error B")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/err-a", nil))
	r.ServeHTTP(w, httptest.NewRequest("GET", "/err-b", nil))

	m := r.GetMetrics()
	if len(m.RecentErrors) != 2 {
		t.Fatalf("want 2 recent errors, got %d", len(m.RecentErrors))
	}
	if m.RecentErrors[0].Code != 400 || m.RecentErrors[0].Message != "error A" {
		t.Fatalf("first recent error wrong: %+v", m.RecentErrors[0])
	}
	if m.RecentErrors[1].Code != 403 || m.RecentErrors[1].Message != "error B" {
		t.Fatalf("second recent error wrong: %+v", m.RecentErrors[1])
	}
	if m.RecentErrors[0].Method != "GET" || m.RecentErrors[0].Path != "/err-a" {
		t.Fatalf("recent error should carry path/method: %+v", m.RecentErrors[0])
	}
}

func TestHEAD_FallbackToGET_NoBody(t *testing.T) {
	r := New()

	r.GET("/resource", func(c *ctx.Ctx) {
		c.JSON(http.StatusOK, map[string]string{"hello": "world", "extra": "data"})
	})
	r.GET("/empty", func(c *ctx.Ctx) {
		c.Status(http.StatusNoContent)
	})

	t.Run("HEAD_uses_GET_handler_no_body", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("HEAD", "/resource", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("HEAD should be 200, got %d", w.Code)
		}
		if w.Body.Len() != 0 {
			t.Fatalf("HEAD response must have empty body, got %d bytes: %s",
				w.Body.Len(), w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("HEAD should copy content-type header, got %q", ct)
		}
	})

	t.Run("HEAD_to_noContent", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("HEAD", "/empty", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("HEAD should respect handler status, got %d", w.Code)
		}
	})
}

func TestOPTIONS_AutoCORS_WithoutRegister(t *testing.T) {
	r := New()

	r.GET("/data", func(c *ctx.Ctx) {})

	t.Run("OPTIONS_registered_path_returns_204_with_CORS_headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("OPTIONS", "/data", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("OPTIONS: want 204, got %d", w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("missing Allow-Origin, got: %+v", w.Header())
		}
		methods := w.Header().Get("Access-Control-Allow-Methods")
		if !strings.Contains(methods, "GET") || !strings.Contains(methods, "POST") {
			t.Fatalf("Allow-Methods should include GET+POST, got %q", methods)
		}
		if w.Header().Get("Access-Control-Allow-Headers") == "" {
			t.Fatal("missing Allow-Headers header")
		}
	})

	t.Run("OPTIONS_arbitrary_path_also_CORS", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("OPTIONS", "/never-registered/xyz", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("OPTIONS any path: want 204, got %d", w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("missing Allow-Origin for unregistered path")
		}
	})
}

func TestAuth_Middleware_BearerAdmin_Success(t *testing.T) {
	r := New()
	r.SetErrorHandler(func(c *ctx.Ctx, err error) {
		appErr := errors.FromError(err)
		c.JSON(appErr.Code, appErr)
	})

	api := r.Group("/api")
	api.Use(middleware.Auth(""))

	admin := api.Group("/admin", middleware.Auth("admin"))
	admin.GET("/dashboard", func(c *ctx.Ctx) {
		c.JSON(http.StatusOK, map[string]string{"secret": "admin-only"})
	})

	t.Run("no_token_401", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/admin/dashboard", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("no token: want 401, got %d", w.Code)
		}
	})

	t.Run("bearer_alice_to_admin_403", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/admin/dashboard", nil)
		req.Header.Set("Authorization", "Bearer alice")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("alice to admin: want 403, got %d", w.Code)
		}
		var resp errors.AppError
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("not valid JSON: %v", err)
		}
		if resp.Code != http.StatusForbidden {
			t.Fatalf("error body code want 403, got %d", resp.Code)
		}
	})

	t.Run("bearer_admin_to_admin_200", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/admin/dashboard", nil)
		req.Header.Set("Authorization", "Bearer admin")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("admin to admin: want 200, got %d, body=%s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "admin-only") {
			t.Fatalf("want admin-only content, got %s", w.Body.String())
		}
	})
}

func TestErrorHandler_RequestIdMethodPath(t *testing.T) {
	r := New()
	r.Use(middleware.RequestID())

	r.SetErrorHandler(func(c *ctx.Ctx, err error) {
		appErr := errors.FromError(err)
		rid, _ := c.Get("request_id")
		c.JSON(appErr.Code, map[string]interface{}{
			"code":       appErr.Code,
			"message":    appErr.Message,
			"request_id": rid,
			"method":     c.Method(),
			"path":       c.Path(),
		})
	})

	r.GET("/boom", func(c *ctx.Ctx) {
		c.NotFound("boom gone")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/boom", nil)
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v, body=%s", err, w.Body.String())
	}
	if resp["method"] != "GET" {
		t.Fatalf("want method=GET, got %v", resp["method"])
	}
	if resp["path"] != "/boom" {
		t.Fatalf("want path=/boom, got %v", resp["path"])
	}
	if resp["request_id"] == nil || resp["request_id"] == "" {
		t.Fatalf("want non-empty request_id, got %v", resp["request_id"])
	}
	if resp["code"].(float64) != 404 {
		t.Fatalf("want code=404, got %v", resp["code"])
	}
}

func TestGetRequests_RequestId_Duration(t *testing.T) {
	r := New()
	r.Use(middleware.RequestID())

	r.GET("/ok", func(c *ctx.Ctx) {
		c.String(http.StatusOK, "ok")
	})
	r.GET("/err", func(c *ctx.Ctx) {
		c.BadRequest("nope")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/ok", nil))
	r.ServeHTTP(w, httptest.NewRequest("GET", "/err", nil))
	r.ServeHTTP(w, httptest.NewRequest("GET", "/never-exists", nil))

	all := r.GetRequests()
	if len(all) != 3 {
		t.Fatalf("want 3 recent requests, got %d", len(all))
	}

	t.Run("status_and_path", func(t *testing.T) {
		if all[0].StatusCode != 200 || all[0].Path != "/ok" {
			t.Fatalf("req[0] wrong: %+v", all[0])
		}
		if all[1].StatusCode != 400 || all[1].Path != "/err" || !all[1].HasError {
			t.Fatalf("req[1] wrong: %+v", all[1])
		}
		if all[2].StatusCode != 404 || all[2].Path != "/never-exists" {
			t.Fatalf("req[2] wrong: %+v", all[2])
		}
	})

	t.Run("request_id_matches_middleware", func(t *testing.T) {
		for i, req := range all {
			if req.RequestID == "" {
				t.Fatalf("req[%d] has empty request_id", i)
			}
		}
	})

	t.Run("duration_recorded", func(t *testing.T) {
		for i, req := range all {
			if req.DurationMs < 0 {
				t.Fatalf("req[%d] negative duration: %d", i, req.DurationMs)
			}
		}
	})

	t.Run("error_message_captured", func(t *testing.T) {
		if all[1].ErrorMsg != "nope" {
			t.Fatalf("req[1] ErrorMsg want 'nope', got %q", all[1].ErrorMsg)
		}
	})
}

func TestMetrics_Accurate_Count_WithAllRequestTypes(t *testing.T) {
	r := New()

	r.GET("/ok", func(c *ctx.Ctx) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	for i := 0; i < 5; i++ {
		r.ServeHTTP(w, httptest.NewRequest("GET", "/ok", nil))
	}
	for i := 0; i < 3; i++ {
		r.ServeHTTP(w, httptest.NewRequest("GET", "/not-here", nil))
	}
	r.ServeHTTP(w, httptest.NewRequest("OPTIONS", "/anything", nil))
	r.ServeHTTP(w, httptest.NewRequest("OPTIONS", "/not-here-either", nil))
	r.ServeHTTP(w, httptest.NewRequest("HEAD", "/ok", nil))

	m := r.GetMetrics()

	expectedTotal := uint64(5 + 3 + 2 + 1)
	if m.TotalRequests != expectedTotal {
		t.Fatalf("total_requests: want %d, got %d", expectedTotal, m.TotalRequests)
	}

	if m.StatusCounts["200"] != 6 {
		t.Fatalf("200 count: want 6 (5xGET + 1xHEAD), got %d", m.StatusCounts["200"])
	}
	if m.StatusCounts["404"] != 3 {
		t.Fatalf("404 count: want 3, got %d", m.StatusCounts["404"])
	}
	if m.StatusCounts["204"] != 2 {
		t.Fatalf("204 (OPTIONS) count: want 2, got %d", m.StatusCounts["204"])
	}

	totalStatus := 0
	for _, v := range m.StatusCounts {
		totalStatus += v
	}
	if uint64(totalStatus) != expectedTotal {
		t.Fatalf("sum(status_counts)=%d != total_requests=%d (diff=%d)",
			totalStatus, expectedTotal, expectedTotal-uint64(totalStatus))
	}
}

func TestMetrics_Consecutive404_ShowUp(t *testing.T) {
	r := New()
	for i := 0; i < 4; i++ {
		w := httptest.NewRecorder()
		path := fmt.Sprintf("/ghost-%d", i)
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	}

	m := r.GetMetrics()
	if m.StatusCounts["404"] != 4 {
		t.Fatalf("want 4x404, got %d", m.StatusCounts["404"])
	}
	if len(m.RecentErrors) < 4 {
		t.Fatalf("want >=4 recent errors, got %d", len(m.RecentErrors))
	}
	reqs := r.GetRequests()
	if len(reqs) != 4 {
		t.Fatalf("want 4 requests, got %d", len(reqs))
	}
	for _, rq := range reqs {
		if rq.StatusCode != 404 || !rq.HasError {
			t.Fatalf("all should be 404 errors, got %+v", rq)
		}
	}
}

func TestListRoutesFiltered_AllDimensions(t *testing.T) {
	r := New()
	r.Use(middleware.Logger())

	r.GET("/public/home", func(c *ctx.Ctx) {})
	r.POST("/public/contact", func(c *ctx.Ctx) {})

	api := r.Group("/api", middleware.Auth(""))
	api.GET("/me", func(c *ctx.Ctx) {})
	api.POST("/orders", func(c *ctx.Ctx) {})

	admin := api.Group("/admin", middleware.Auth("admin"))
	admin.GET("/stats", func(c *ctx.Ctx) {})

	t.Run("filter_method_only", func(t *testing.T) {
		posts := r.ListRoutesFiltered(RouteFilter{Method: "POST"})
		if len(posts) != 2 {
			t.Fatalf("want 2 POST routes, got %d", len(posts))
		}
		for _, rt := range posts {
			if rt.Method != "POST" {
				t.Fatalf("non-POST in result: %+v", rt)
			}
		}
	})

	t.Run("filter_prefix_api", func(t *testing.T) {
		apiRoutes := r.ListRoutesFiltered(RouteFilter{PathPrefix: "/api"})
		if len(apiRoutes) != 3 {
			t.Fatalf("want 3 /api routes, got %d", len(apiRoutes))
		}
		for _, rt := range apiRoutes {
			if !strings.HasPrefix(rt.Path, "/api") {
				t.Fatalf("non-api in result: %+v", rt)
			}
		}
	})

	t.Run("filter_prefix_admin_nested", func(t *testing.T) {
		adminRoutes := r.ListRoutesFiltered(RouteFilter{PathPrefix: "/api/admin"})
		if len(adminRoutes) != 1 {
			t.Fatalf("want 1 admin route, got %d", len(adminRoutes))
		}
		if adminRoutes[0].Path != "/api/admin/stats" {
			t.Fatalf("want /api/admin/stats, got %s", adminRoutes[0].Path)
		}
		if adminRoutes[0].GroupMwCount != 2 {
			t.Fatalf("admin route should have 2 group mw, got %d", adminRoutes[0].GroupMwCount)
		}
	})

	t.Run("filter_middleware_auth", func(t *testing.T) {
		withAuth := r.ListRoutesFiltered(RouteFilter{ContainsMiddleware: "Auth"})
		if len(withAuth) != 3 {
			t.Fatalf("want 3 routes with Auth mw, got %d: %+v", len(withAuth), withAuth)
		}
	})

	t.Run("filter_method_and_prefix", func(t *testing.T) {
		res := r.ListRoutesFiltered(RouteFilter{Method: "GET", PathPrefix: "/public"})
		if len(res) != 1 || res[0].Path != "/public/home" {
			t.Fatalf("want [GET /public/home], got %+v", res)
		}
	})

	t.Run("filter_nomatch", func(t *testing.T) {
		res := r.ListRoutesFiltered(RouteFilter{PathPrefix: "/nope"})
		if len(res) != 0 {
			t.Fatalf("want empty, got %d", len(res))
		}
	})
}

func TestPrintRoutesFiltered_ShowsFilterHeader(t *testing.T) {
	r := New()
	r.Use(middleware.RequestID())
	r.GET("/x", func(c *ctx.Ctx) {})

	all := r.PrintRoutes()
	if !strings.Contains(all, "Registered Routes") {
		t.Fatalf("PrintRoutes should contain header, got:\n%s", all)
	}

	filtered := r.PrintRoutesFiltered(RouteFilter{PathPrefix: "/api"})
	if !strings.Contains(filtered, "(no routes matched)") {
		t.Fatalf("PrintRoutesFiltered should say no matches, got:\n%s", filtered)
	}
	if !strings.Contains(filtered, "prefix=") {
		t.Fatalf("PrintRoutesFiltered should show filter info, got:\n%s", filtered)
	}
}

func TestDebugRoutes_NeedBearerAdmin(t *testing.T) {
	r := New()
	r.SetErrorHandler(func(c *ctx.Ctx, err error) {
		appErr := errors.FromError(err)
		c.JSON(appErr.Code, appErr)
	})

	debug := r.Group("/debug", middleware.Auth("admin"))
	debug.GET("/metrics", func(c *ctx.Ctx) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	api := r.Group("/api", middleware.Auth(""))
	api.GET("/health", func(c *ctx.Ctx) {
		c.String(http.StatusOK, "ok")
	})

	t.Run("debug_no_token_401", func(t *testing.T) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/debug/metrics", nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("debug no token: want 401, got %d", w.Code)
		}
	})

	t.Run("debug_user_403", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/debug/metrics", nil)
		req.Header.Set("Authorization", "Bearer alice")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("debug user: want 403, got %d", w.Code)
		}
	})

	t.Run("debug_admin_200", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/debug/metrics", nil)
		req.Header.Set("Authorization", "Bearer admin")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("debug admin: want 200, got %d", w.Code)
		}
	})

	t.Run("api_user_200_not_affected", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/health", nil)
		req.Header.Set("Authorization", "Bearer alice")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("api user should still work, got %d", w.Code)
		}
	})
}
