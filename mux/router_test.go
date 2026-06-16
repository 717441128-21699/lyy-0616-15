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
