package main

import (
	"fmt"
	"log"
	"net/http"

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

func main() {
	r := mux.Default()

	r.Use(middleware.CORS("*"))
	r.Use(middleware.Timing())

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
	api.Use(middleware.Auth("user"))

	api.GET("/profile", profileHandler)
	api.GET("/settings", settingsHandler)

	orders := api.Group("/orders")
	orders.GET("", listOrdersHandler)
	orders.GET("/:id", getOrderHandler)
	orders.POST("", createOrderHandler)

	admin := api.Group("/admin", func(c *ctx.Ctx) {
		log.Println("[admin MW] checking admin role")
		authVal, ok := c.Get("auth")
		if !ok {
			c.Forbidden("admin access required")
			c.Abort()
			return
		}
		authInfo, ok := authVal.(*middleware.AuthInfo)
		if !ok || authInfo.Role != "admin" {
			c.Forbidden("admin role required")
			c.Abort()
			return
		}
		c.Next()
	})
	admin.GET("/dashboard", dashboardHandler)
	admin.GET("/users", adminListUsersHandler)

	errorsDemo := r.Group("/demo/errors")
	errorsDemo.GET("/bad-request", demoBadRequest)
	errorsDemo.GET("/not-found", demoNotFound)
	errorsDemo.GET("/forbidden", demoForbidden)
	errorsDemo.GET("/validation", demoValidation)
	errorsDemo.GET("/conflict", demoConflict)
	errorsDemo.GET("/custom", demoCustomError)

	fmt.Println("========================================")
	fmt.Println("  Vine Framework - Example Application  ")
	fmt.Println("  Listening on :8080                    ")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("=== 基础路由 ===")
	fmt.Println("  GET    /                    - 首页")
	fmt.Println("  GET    /health              - 健康检查")
	fmt.Println("  GET    /panic               - Panic 恢复测试")
	fmt.Println()
	fmt.Println("=== 用户 CRUD (参数路由) ===")
	fmt.Println("  GET    /users               - 列表")
	fmt.Println("  GET    /users/:id           - 详情")
	fmt.Println("  POST   /users               - 创建 (JSON body)")
	fmt.Println("  PUT    /users/:id           - 更新")
	fmt.Println("  DELETE /users/:id           - 删除")
	fmt.Println()
	fmt.Println("=== 通配符路由 ===")
	fmt.Println("  GET    /files/*filepath     - 文件路径通配")
	fmt.Println()
	fmt.Println("=== 嵌套路由组 + 中间件 ===")
	fmt.Println("  GET    /api/v1/profile           - 需要登录 (Bearer token)")
	fmt.Println("  GET    /api/v1/settings          - 需要登录")
	fmt.Println("  GET    /api/v1/orders            - 需要登录")
	fmt.Println("  GET    /api/v1/orders/:id        - 需要登录")
	fmt.Println("  POST   /api/v1/orders            - 需要登录")
	fmt.Println("  GET    /api/v1/admin/dashboard   - 需要 admin 角色")
	fmt.Println("  GET    /api/v1/admin/users       - 需要 admin 角色")
	fmt.Println()
	fmt.Println("=== 错误处理演示 ===")
	fmt.Println("  GET    /demo/errors/bad-request   - 400 BadRequest()")
	fmt.Println("  GET    /demo/errors/not-found     - 404 NotFound()")
	fmt.Println("  GET    /demo/errors/forbidden     - 403 Forbidden()")
	fmt.Println("  GET    /demo/errors/validation    - 422 ValidationError()")
	fmt.Println("  GET    /demo/errors/conflict      - 409 Conflict()")
	fmt.Println("  GET    /demo/errors/custom        - 自定义 AppError")
	fmt.Println()
	fmt.Println("=== 静态 vs 参数路由优先级 ===")
	fmt.Println("  GET    /users/me                 - 静态优先")
	fmt.Println("  GET    /users/123                - 参数兜底")
	fmt.Println()
	fmt.Println("提示: 使用 Authorization: Bearer admin 可获得 admin 角色")
	fmt.Println()

	if err := http.ListenAndServe(":8080", r); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func homeHandler(c *ctx.Ctx) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"framework": "Vine",
		"version":   "1.0.0",
		"message":   "Welcome to Vine Web Framework",
		"features": []string{
			"Radix tree 路由 (静态 map 索引 O(1))",
			"路径参数 + 通配符 + 回溯匹配",
			"洋葱模型中间件 (全局+组级+嵌套)",
			"Context 数据安全传递",
			"参数绑定与校验",
			"统一错误处理",
			"Panic 自动恢复",
		},
	})
}

func healthHandler(c *ctx.Ctx) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "healthy",
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
		"data":  users,
		"page":  page,
		"limit": limit,
		"total": len(users),
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

	c.JSON(http.StatusOK, found)
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

	c.JSON(http.StatusCreated, user)
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

	c.JSON(http.StatusOK, found)
}

func deleteUserHandler(c *ctx.Ctx) {
	id := c.Param("id")

	for i, u := range users {
		if fmt.Sprintf("%d", u.ID) == id {
			users = append(users[:i], users[i+1:]...)
			c.JSON(http.StatusOK, map[string]string{"message": "user deleted"})
			return
		}
	}

	c.NotFoundf("user %s not found", id)
}

func fileHandler(c *ctx.Ctx) {
	filepath := c.Param("filepath")
	c.JSON(http.StatusOK, map[string]interface{}{
		"filepath": filepath,
		"message":  fmt.Sprintf("Accessing file: %s", filepath),
	})
}

func profileHandler(c *ctx.Ctx) {
	authVal, _ := c.Get("auth")
	authInfo := authVal.(*middleware.AuthInfo)

	c.JSON(http.StatusOK, map[string]interface{}{
		"user_id":  authInfo.UserID,
		"role":     authInfo.Role,
		"verified": authInfo.Verified,
		"message":  "This is your profile (api/v1 group)",
	})
}

func settingsHandler(c *ctx.Ctx) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"theme":   "dark",
		"lang":    "zh-CN",
		"message": "Your settings (api/v1 group)",
	})
}

func listOrdersHandler(c *ctx.Ctx) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"orders": []map[string]interface{}{
			{"id": 1, "product": "laptop", "amount": 5999},
			{"id": 2, "product": "phone", "amount": 2999},
		},
		"total": 2,
	})
}

func getOrderHandler(c *ctx.Ctx) {
	id := c.Param("id")
	c.JSON(http.StatusOK, map[string]interface{}{
		"id":       id,
		"product":  "laptop",
		"amount":   5999,
		"status":   "shipped",
		"message":  "订单详情 (nested orders group)",
	})
}

func createOrderHandler(c *ctx.Ctx) {
	c.JSON(http.StatusCreated, map[string]interface{}{
		"id":      999,
		"status":  "created",
		"message": "订单创建成功",
	})
}

func dashboardHandler(c *ctx.Ctx) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"total_users":   len(users),
		"total_orders":  2,
		"revenue":       8998,
		"uptime":        "running",
		"message":       "Admin dashboard - 你拥有 admin 权限",
	})
}

func adminListUsersHandler(c *ctx.Ctx) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"users":   users,
		"message": "管理员视角的用户列表",
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
			"field":   "age",
			"value":   -1,
			"reason":  "年龄必须大于0",
		},
	)
	c.Error(err)
}
