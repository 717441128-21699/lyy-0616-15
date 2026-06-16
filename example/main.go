package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/trae-framework/vine/ctx"
	appErrors "github.com/trae-framework/vine/errors"
	"github.com/trae-framework/vine/middleware"
	"github.com/trae-framework/vine/mux"
)

type CreateUserReq struct {
	Name  string `json:"name"  form:"name"  required:"true" min:"2" max:"50"`
	Email string `json:"email" form:"email" required:"true" min:"5" max:"100"`
	Age   int    `json:"age"   form:"age"   required:"true" min:"1" max:"150"`
}

type UpdateUserReq struct {
	Name string `json:"name" form:"name" min:"2" max:"50"`
	Age  int    `json:"age"  form:"age"  min:"1" max:"150"`
}

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

var users = []User{
	{ID: 1, Name: "Alice", Email: "alice@example.com", Age: 28},
	{ID: 2, Name: "Bob", Email: "bob@example.com", Age: 35},
}

type unifiedErrorResponse struct {
	Success   bool        `json:"success"`
	RequestID string      `json:"request_id,omitempty"`
	Path      string      `json:"path,omitempty"`
	Method    string      `json:"method,omitempty"`
	Timestamp int64       `json:"timestamp,omitempty"`
	Error     errorDetail `json:"error"`
}

type errorDetail struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Detail  interface{} `json:"detail,omitempty"`
}

func main() {
	r := mux.Default()

	r.SetErrorHandler(func(c *ctx.Ctx, err error) {
		appErr := appErrors.FromError(err)
		reqID, _ := c.Get("request_id")
		reqIDStr, _ := reqID.(string)
		resp := unifiedErrorResponse{
			Success:   false,
			RequestID: reqIDStr,
			Path:      c.Path(),
			Method:    c.Method(),
			Timestamp: time.Now().Unix(),
			Error: errorDetail{
				Code:    appErr.Code,
				Message: appErr.Message,
				Detail:  appErr.Detail,
			},
		}
		c.JSON(appErr.Code, resp)
	})

	r.Use(middleware.CORS("*"))
	r.Use(middleware.Timing())

	debug := r.Group("/debug", middleware.Auth("admin"))
	debug.GET("/routes", func(c *ctx.Ctx) {
		filter := mux.RouteFilter{
			Method:             c.Query("method"),
			PathPrefix:         c.Query("prefix"),
			ContainsMiddleware: c.Query("with_mw"),
		}
		filtered := r.ListRoutesFiltered(filter)
		c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"filter":       filter,
				"routes":       filtered,
				"routes_count": len(filtered),
			},
		})
	})
	debug.GET("/routes/print", func(c *ctx.Ctx) {
		filter := mux.RouteFilter{
			Method:             c.Query("method"),
			PathPrefix:         c.Query("prefix"),
			ContainsMiddleware: c.Query("with_mw"),
		}
		c.String(http.StatusOK, "%s", r.PrintRoutesFiltered(filter))
	})
	debug.GET("/metrics", func(c *ctx.Ctx) {
		m := r.GetMetrics()
		c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    m,
		})
	})
	debug.GET("/requests", func(c *ctx.Ctx) {
		limit := c.QueryInt("limit", 20)
		if limit <= 0 {
			limit = 20
		}
		all := r.GetRequests()
		if limit < len(all) {
			all = all[len(all)-limit:]
		}
		c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"requests":      all,
				"requests_count": len(all),
				"limit":         limit,
			},
		})
	})

	r.GET("/", homeHandler)
	r.GET("/health", healthHandler)
	r.GET("/panic", panicHandler)

	r.GET("/users", listUsersHandler)
	r.GET("/users/:id", getUserHandler)
	r.POST("/users", createUserHandler)
	r.PUT("/users/:id", updateUserHandler)
	r.DELETE("/users/:id", deleteUserHandler)

	r.GET("/files/*filepath", fileHandler)

	api := r.Group("/api/v1")
	api.Use(middleware.Auth(""))

	api.GET("/profile", profileHandler)
	api.GET("/settings", settingsHandler)

	orders := api.Group("/orders")
	orders.GET("", listOrdersHandler)
	orders.GET("/:id", getOrderHandler)
	orders.POST("", createOrderHandler)

	admin := api.Group("/admin", middleware.Auth("admin"))
	admin.GET("/dashboard", dashboardHandler)
	admin.GET("/users", adminListUsersHandler)

	errorsDemo := r.Group("/demo/errors")
	errorsDemo.GET("/bad-request", demoBadRequest)
	errorsDemo.GET("/not-found", demoNotFound)
	errorsDemo.GET("/forbidden", demoForbidden)
	errorsDemo.GET("/validation", demoValidation)
	errorsDemo.GET("/conflict", demoConflict)
	errorsDemo.GET("/custom", demoCustomError)
	errorsDemo.GET("/plain-error", demoPlainError)
	errorsDemo.GET("/panic", demoPanic)

	fmt.Println("========================================")
	fmt.Println("  Vine Framework - Example Application  ")
	fmt.Println("  Listening on :8080                    ")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println(r.PrintRoutes())
	fmt.Println()
	fmt.Println("=== 调试接口 (全部需要 Bearer admin, 状态码 401/403) ===")
	fmt.Println("  GET    /debug/routes?method=GET&prefix=/api&with_mw=Auth")
	fmt.Println("         - 支持过滤: method / prefix / with_mw (按中间件名包含匹配)")
	fmt.Println("  GET    /debug/routes/print?prefix=/api/v1/admin")
	fmt.Println("         - 命令行友好的表格, 支持同样的过滤参数")
	fmt.Println("  GET    /debug/metrics")
	fmt.Println("         - route_count / total_requests / status_counts / recent_errors / uptime")
	fmt.Println("  GET    /debug/requests?limit=30")
	fmt.Println("         - 最近请求: request_id / method / path / status / duration_ms / has_error")
	fmt.Println()
	fmt.Println("=== 基础路由 ===")
	fmt.Println("  GET    /                           - 首页")
	fmt.Println("  GET    /health                     - 健康检查")
	fmt.Println("  GET    /panic                      - Panic 恢复测试")
	fmt.Println()
	fmt.Println("=== 用户 CRUD ===")
	fmt.Println("  GET    /users                      - 列表")
	fmt.Println("  GET    /users/:id                  - 详情 (NotFound 统一格式, 带 request_id)")
	fmt.Println("  POST   /users                      - 创建")
	fmt.Println("  PUT    /users/:id                  - 更新")
	fmt.Println("  DELETE /users/:id                  - 删除")
	fmt.Println("  HEAD   /users/:id                  - 自动复用 GET handler, 无 body")
	fmt.Println()
	fmt.Println("=== 嵌套路由组 + 中间件 (区分 401/403) ===")
	fmt.Println("  无 token                             → 401 Unauthorized")
	fmt.Println("  Bearer alice (普通用户)              → /api/v1/* 可访问, /api/v1/admin/* → 403")
	fmt.Println("  Bearer admin (管理员)                → 所有接口 (含 /debug/*) 可访问")
	fmt.Println()
	fmt.Println("  GET    /api/v1/profile             - 个人信息")
	fmt.Println("  GET    /api/v1/settings            - 设置")
	fmt.Println("  GET    /api/v1/orders              - 订单列表")
	fmt.Println("  GET    /api/v1/orders/:id          - 订单详情")
	fmt.Println("  POST   /api/v1/orders              - 创建订单")
	fmt.Println("  GET    /api/v1/admin/dashboard     - 管理后台")
	fmt.Println("  GET    /api/v1/admin/users         - 用户管理")
	fmt.Println()
	fmt.Println("=== CORS 预检 (OPTIONS, 无需手动注册) ===")
	fmt.Println("  OPTIONS /any/route                 → 204 No Content + CORS 头")
	fmt.Println()
	fmt.Println("=== 错误处理演示 (全部同一 JSON 结构) ===")
	fmt.Println("  GET    /demo/errors/bad-request    - 400")
	fmt.Println("  GET    /demo/errors/not-found      - 404")
	fmt.Println("  GET    /demo/errors/forbidden      - 403")
	fmt.Println("  GET    /demo/errors/validation     - 422")
	fmt.Println("  GET    /demo/errors/conflict       - 409")
	fmt.Println("  GET    /demo/errors/custom         - 400 + detail")
	fmt.Println("  GET    /demo/errors/plain-error    - 普通 Go error → 500")
	fmt.Println("  GET    /demo/errors/panic          - panic → Recovery → 500")
	fmt.Println()
	fmt.Println("  Curl 示例:")
	fmt.Println("    # 401 没登录访问 debug")
	fmt.Println("    curl -i http://localhost:8080/debug/metrics")
	fmt.Println("    # 403 普通用户访问 debug")
	fmt.Println("    curl -i -H 'Authorization: Bearer alice' http://localhost:8080/debug/metrics")
	fmt.Println("    # 200 admin 访问 debug")
	fmt.Println("    curl -s -H 'Authorization: Bearer admin' http://localhost:8080/debug/metrics | python -m json.tool")
	fmt.Println("    # 路由过滤: 只看带 Auth 的 /api 路由")
	fmt.Println("    curl -s -H 'Authorization: Bearer admin' 'http://localhost:8080/debug/routes/print?prefix=/api&with_mw=Auth'")
	fmt.Println("    # 看最近请求 (访问几个错误接口后更好看)")
	fmt.Println("    curl -s -H 'Authorization: Bearer admin' http://localhost:8080/debug/requests | python -m json.tool")
	fmt.Println()

	if err := http.ListenAndServe(":8080", r); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func homeHandler(c *ctx.Ctx) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"framework": "Vine",
			"version":   "1.0.0",
			"message":   "Welcome to Vine Web Framework",
			"features":  strings.Join([]string{
				"Radix tree 路由 (静态 map 索引 O(1))",
				"路径参数 + 通配符 + 回溯匹配",
				"洋葱模型中间件 (全局+组级+嵌套)",
				"Context 数据安全传递",
				"参数绑定与校验",
				"统一错误处理 (自定义全局 ErrorHandler, 带 request_id/method/path)",
				"Panic 自动恢复",
				"HEAD 自动复用 GET, OPTIONS 预检自动 CORS",
				"请求追踪 /debug/requests (request_id 贯穿日志)",
				"指标 /debug/metrics (真实口径: 404/预检/HEAD 全计入)",
				"路由过滤 /debug/routes?method/prefix/with_mw",
			}, "\n              • "),
		},
	})
}

func healthHandler(c *ctx.Ctx) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"success":    true,
		"status":     "healthy",
		"request_id": c.GetString("request_id"),
	})
}

func panicHandler(c *ctx.Ctx) {
	panic("something went terribly wrong!")
}

func listUsersHandler(c *ctx.Ctx) {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    users,
		"page":    page,
		"limit":   limit,
		"total":   len(users),
	})
}

func getUserHandler(c *ctx.Ctx) {
	id := c.Param("id")
	if id == "" {
		c.BadRequest("missing user id")
		return
	}

	var found *User
	for i := range users {
		if fmt.Sprintf("%d", users[i].ID) == id {
			found = &users[i]
			break
		}
	}

	if found == nil {
		c.NotFoundf("user %s not found", id)
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    found,
	})
}

func createUserHandler(c *ctx.Ctx) {
	var req CreateUserReq
	if err := c.Bind(&req); err != nil {
		c.BadRequestf("invalid request: %v", err)
		return
	}

	if err := c.Validate(&req); err != nil {
		c.ValidationError(err.Error())
		return
	}

	for _, u := range users {
		if u.Email == req.Email {
			c.Conflict("email already exists")
			return
		}
	}

	user := User{
		ID:    len(users) + 1,
		Name:  req.Name,
		Email: req.Email,
		Age:   req.Age,
	}
	users = append(users, user)

	c.JSON(http.StatusCreated, map[string]interface{}{
		"success": true,
		"data":    user,
	})
}

func updateUserHandler(c *ctx.Ctx) {
	id := c.Param("id")

	var found *User
	for i := range users {
		if fmt.Sprintf("%d", users[i].ID) == id {
			found = &users[i]
			break
		}
	}

	if found == nil {
		c.NotFoundf("user %s not found", id)
		return
	}

	var req UpdateUserReq
	if err := c.Bind(&req); err != nil {
		c.BadRequestf("invalid request: %v", err)
		return
	}

	if req.Name != "" {
		found.Name = req.Name
	}
	if req.Age > 0 {
		found.Age = req.Age
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    found,
	})
}

func deleteUserHandler(c *ctx.Ctx) {
	id := c.Param("id")

	for i, u := range users {
		if fmt.Sprintf("%d", u.ID) == id {
			users = append(users[:i], users[i+1:]...)
			c.JSON(http.StatusOK, map[string]interface{}{
				"success": true,
				"message": "user deleted",
			})
			return
		}
	}

	c.NotFoundf("user %s not found", id)
}

func fileHandler(c *ctx.Ctx) {
	filepath := c.Param("filepath")
	c.JSON(http.StatusOK, map[string]interface{}{
		"success":  true,
		"filepath": filepath,
		"message":  fmt.Sprintf("Accessing file: %s", filepath),
	})
}

func profileHandler(c *ctx.Ctx) {
	authVal, _ := c.Get("auth")
	authInfo := authVal.(*middleware.AuthInfo)

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"user_id":  authInfo.UserID,
			"role":     authInfo.Role,
			"verified": authInfo.Verified,
			"message":  "This is your profile (api/v1 group)",
		},
	})
}

func settingsHandler(c *ctx.Ctx) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"theme":   "dark",
			"lang":    "zh-CN",
			"message": "Your settings (api/v1 group)",
		},
	})
}

func listOrdersHandler(c *ctx.Ctx) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data": []map[string]interface{}{
			{"id": 1, "product": "laptop", "amount": 5999},
			{"id": 2, "product": "phone", "amount": 2999},
		},
		"total": 2,
	})
}

func getOrderHandler(c *ctx.Ctx) {
	id := c.Param("id")
	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"id":       id,
			"product":  "laptop",
			"amount":   5999,
			"status":   "shipped",
			"message":  "订单详情 (nested orders group)",
		},
	})
}

func createOrderHandler(c *ctx.Ctx) {
	var body map[string]interface{}
	if err := json.NewDecoder(c.Request.Body).Decode(&body); err != nil {
		c.BadRequestf("invalid json body: %v", err)
		return
	}
	c.JSON(http.StatusCreated, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"id":      999,
			"status":  "created",
			"message": "订单创建成功",
			"body":    body,
		},
	})
}

func dashboardHandler(c *ctx.Ctx) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"total_users":  len(users),
			"total_orders": 2,
			"revenue":      8998,
			"uptime":       "running",
			"message":      "Admin dashboard - 你拥有 admin 权限 ✓",
		},
	})
}

func adminListUsersHandler(c *ctx.Ctx) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"users":   users,
			"message": "管理员视角的用户列表",
		},
	})
}

func demoBadRequest(c *ctx.Ctx) {
	c.BadRequest("缺少必要参数: user_id")
}

func demoNotFound(c *ctx.Ctx) {
	c.NotFound("您访问的资源不存在")
}

func demoForbidden(c *ctx.Ctx) {
	c.Forbidden("您没有权限访问此资源")
}

func demoValidation(c *ctx.Ctx) {
	c.ValidationError("字段 'email' 格式不正确")
}

func demoConflict(c *ctx.Ctx) {
	c.Conflict("资源已存在，无法重复创建")
}

func demoCustomError(c *ctx.Ctx) {
	err := appErrors.WithDetail(
		appErrors.ErrBadRequest,
		map[string]interface{}{
			"field":  "age",
			"value":  -1,
			"reason": "年龄必须大于0",
		},
	)
	c.Error(err)
}

func demoPlainError(c *ctx.Ctx) {
	err := fmt.Errorf("database connection lost: timeout after 30s")
	c.HandleError(err)
}

func demoPanic(c *ctx.Ctx) {
	panic("oops! something unexpected happened in business logic")
}
