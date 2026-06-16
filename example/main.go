package main

import (
	"fmt"
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

	api := r.Group("/api")
	api.Use(middleware.Auth("user"))
	api.GET("/profile", profileHandler)
	api.GET("/settings", settingsHandler)

	admin := r.Group("/admin")
	admin.Use(middleware.Auth("admin"))
	admin.GET("/dashboard", dashboardHandler)

	fmt.Println("========================================")
	fmt.Println("  Vine Framework - Example Application  ")
	fmt.Println("  Listening on :8080                    ")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("Routes:")
	fmt.Println("  GET    /                    - Home")
	fmt.Println("  GET    /health              - Health check")
	fmt.Println("  GET    /panic               - Panic recovery test")
	fmt.Println("  GET    /users               - List users")
	fmt.Println("  GET    /users/:id           - Get user by ID")
	fmt.Println("  POST   /users               - Create user (JSON body)")
	fmt.Println("  PUT    /users/:id           - Update user")
	fmt.Println("  DELETE /users/:id           - Delete user")
	fmt.Println("  GET    /files/*filepath     - Wildcard file path")
	fmt.Println("  GET    /api/profile         - Auth-protected profile")
	fmt.Println("  GET    /api/settings        - Auth-protected settings")
	fmt.Println("  GET    /admin/dashboard     - Admin-only dashboard")
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
		"endpoints": []string{
			"GET /health",
			"GET /panic",
			"GET /users",
			"GET /users/:id",
			"POST /users",
			"PUT /users/:id",
			"DELETE /users/:id",
			"GET /files/*filepath",
			"GET /api/profile (auth required)",
			"GET /admin/dashboard (admin required)",
		},
	})
}

func healthHandler(c *ctx.Ctx) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "healthy",
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
		err := appErrors.WithDetail(appErrors.ErrBadRequest, "missing user id")
		c.JSON(err.Code, err)
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
		err := appErrors.WithDetail(appErrors.ErrNotFound, map[string]string{"user_id": id})
		c.JSON(err.Code, err)
		return
	}

	c.JSON(http.StatusOK, found)
}

func createUserHandler(c *ctx.Ctx) {
	var req CreateUserReq
	if err := c.Bind(&req); err != nil {
		appErr := appErrors.WithDetail(appErrors.ErrBadRequest, err.Error())
		c.JSON(appErr.Code, appErr)
		return
	}

	if err := c.Validate(&req); err != nil {
		appErr := appErrors.WithDetail(appErrors.ErrValidation, err.Error())
		c.JSON(appErr.Code, appErr)
		return
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
		err := appErrors.WithDetail(appErrors.ErrNotFound, map[string]string{"user_id": id})
		c.JSON(err.Code, err)
		return
	}

	var req UpdateUserReq
	if err := c.Bind(&req); err != nil {
		appErr := appErrors.WithDetail(appErrors.ErrBadRequest, err.Error())
		c.JSON(appErr.Code, appErr)
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

	err := appErrors.WithDetail(appErrors.ErrNotFound, map[string]string{"user_id": id})
	c.JSON(err.Code, err)
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
	authInfo, ok := authVal.(*middleware.AuthInfo)
	if !ok {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "auth info not found"})
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"user_id":  authInfo.UserID,
		"role":     authInfo.Role,
		"verified": authInfo.Verified,
		"message":  "This is your profile",
	})
}

func settingsHandler(c *ctx.Ctx) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"theme":   "dark",
		"lang":    "zh-CN",
		"message": "Your settings",
	})
}

func dashboardHandler(c *ctx.Ctx) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"total_users": len(users),
		"uptime":      "running",
		"message":     "Admin dashboard - you have admin privileges",
	})
}
