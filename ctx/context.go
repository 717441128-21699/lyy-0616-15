package ctx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/trae-framework/vine/errors"
	"github.com/trae-framework/vine/radix"
)

type Ctx struct {
	Request  *http.Request
	Writer   http.ResponseWriter
	params   radix.Params
	store    map[string]interface{}
	status   int
	written  bool
	handlers []HandlerFunc
	index    int
	mu       sync.RWMutex
}

var ctxPool = sync.Pool{
	New: func() interface{} {
		return &Ctx{
			store: make(map[string]interface{}),
		}
	},
}

func New(w http.ResponseWriter, r *http.Request) *Ctx {
	c := ctxPool.Get().(*Ctx)
	c.Request = r
	c.Writer = w
	c.params = nil
	c.status = 0
	c.written = false
	c.index = -1
	c.handlers = nil
	for k := range c.store {
		delete(c.store, k)
	}
	return c
}

func (c *Ctx) Release() {
	c.Request = nil
	c.Writer = nil
	c.params = nil
	c.status = 0
	c.written = false
	c.index = -1
	c.handlers = nil
	ctxPool.Put(c)
}

func (c *Ctx) SetHandlers(handlers []HandlerFunc) {
	c.handlers = handlers
	c.index = -1
}

func (c *Ctx) SetParams(params radix.Params) {
	c.params = params
}

func (c *Ctx) Param(name string) string {
	if c.params == nil {
		return ""
	}
	return c.params.ByName(name)
}

func (c *Ctx) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

func (c *Ctx) QueryInt(key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func (c *Ctx) DefaultQuery(key, def string) string {
	v := c.Query(key)
	if v == "" {
		return def
	}
	return v
}

func (c *Ctx) Set(key string, value interface{}) {
	c.mu.Lock()
	c.store[key] = value
	c.mu.Unlock()
}

func (c *Ctx) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	v, ok := c.store[key]
	c.mu.RUnlock()
	return v, ok
}

func (c *Ctx) MustGet(key string) interface{} {
	v, ok := c.Get(key)
	if !ok {
		panic(fmt.Sprintf("key '%s' not found in context", key))
	}
	return v
}

func (c *Ctx) GetString(key string) string {
	v, _ := c.Get(key)
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func (c *Ctx) Status(code int) *Ctx {
	c.status = code
	return c
}

func (c *Ctx) Header(key, value string) {
	c.Writer.Header().Set(key, value)
	return
}

func (c *Ctx) String(code int, format string, values ...interface{}) {
	c.Status(code)
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteHeader(code)
	c.written = true
	c.Writer.Write([]byte(fmt.Sprintf(format, values...)))
}

func (c *Ctx) JSON(code int, obj interface{}) {
	c.Status(code)
	c.Header("Content-Type", "application/json; charset=utf-8")
	data, err := json.Marshal(obj)
	if err != nil {
		c.String(http.StatusInternalServerError, `{"error":"internal server error"}`)
		return
	}
	c.Writer.WriteHeader(code)
	c.written = true
	c.Writer.Write(data)
}

func (c *Ctx) HTML(code int, html string) {
	c.Status(code)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Writer.WriteHeader(code)
	c.written = true
	c.Writer.Write([]byte(html))
}

func (c *Ctx) Error(err *errors.AppError) {
	c.JSON(err.Code, err)
}

func (c *Ctx) Errorf(code int, format string, args ...interface{}) {
	c.JSON(code, errors.Newf(code, format, args...))
}

func (c *Ctx) BadRequest(message string) {
	c.JSON(http.StatusBadRequest, errors.BadRequest(message))
}

func (c *Ctx) BadRequestf(format string, args ...interface{}) {
	c.JSON(http.StatusBadRequest, errors.BadRequestf(format, args...))
}

func (c *Ctx) Unauthorized(message string) {
	c.JSON(http.StatusUnauthorized, errors.Unauthorized(message))
}

func (c *Ctx) Unauthorizedf(format string, args ...interface{}) {
	c.JSON(http.StatusUnauthorized, errors.Unauthorizedf(format, args...))
}

func (c *Ctx) Forbidden(message string) {
	c.JSON(http.StatusForbidden, errors.Forbidden(message))
}

func (c *Ctx) Forbiddenf(format string, args ...interface{}) {
	c.JSON(http.StatusForbidden, errors.Forbiddenf(format, args...))
}

func (c *Ctx) NotFound(message string) {
	c.JSON(http.StatusNotFound, errors.NotFound(message))
}

func (c *Ctx) NotFoundf(format string, args ...interface{}) {
	c.JSON(http.StatusNotFound, errors.NotFoundf(format, args...))
}

func (c *Ctx) InternalServerError(message string) {
	c.JSON(http.StatusInternalServerError, errors.Internal(message))
}

func (c *Ctx) InternalServerErrorf(format string, args ...interface{}) {
	c.JSON(http.StatusInternalServerError, errors.Internalf(format, args...))
}

func (c *Ctx) ValidationError(message string) {
	c.JSON(http.StatusUnprocessableEntity, errors.Validation(message))
}

func (c *Ctx) ValidationErrorf(format string, args ...interface{}) {
	c.JSON(http.StatusUnprocessableEntity, errors.Validationf(format, args...))
}

func (c *Ctx) Conflict(message string) {
	c.JSON(http.StatusConflict, errors.Conflict(message))
}

func (c *Ctx) Bind(obj interface{}) error {
	rv := reflect.ValueOf(obj)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("bind target must be a non-nil pointer")
	}

	ct := c.Request.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		return c.bindJSON(obj)
	}

	return c.bindForm(obj)
}

func (c *Ctx) bindJSON(obj interface{}) error {
	decoder := json.NewDecoder(c.Request.Body)
	return decoder.Decode(obj)
}

func (c *Ctx) bindForm(obj interface{}) error {
	rv := reflect.ValueOf(obj).Elem()
	rt := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		structField := rt.Field(i)

		tag := structField.Tag.Get("form")
		if tag == "" {
			tag = strings.ToLower(structField.Name)
		}
		if tag == "-" {
			continue
		}

		val := c.Request.FormValue(tag)
		if val == "" {
			val = c.Param(tag)
		}
		if val == "" {
			continue
		}

		if err := setField(field, val); err != nil {
			return fmt.Errorf("field '%s': %w", tag, err)
		}
	}
	return nil
}

func (c *Ctx) Validate(obj interface{}) error {
	rv := reflect.ValueOf(obj)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("validate target must be a non-nil pointer")
	}

	rv = rv.Elem()
	rt := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		structField := rt.Field(i)

		required := structField.Tag.Get("required")
		if required == "true" {
			if isZero(field) {
				return fmt.Errorf("field '%s' is required", structField.Name)
			}
		}

		minStr := structField.Tag.Get("min")
		maxStr := structField.Tag.Get("max")

		if minStr != "" || maxStr != "" {
			minVal, _ := strconv.Atoi(minStr)
			maxVal, _ := strconv.Atoi(maxStr)

			switch field.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				v := field.Int()
				if minStr != "" && v < int64(minVal) {
					return fmt.Errorf("field '%s' must be >= %d", structField.Name, minVal)
				}
				if maxStr != "" && v > int64(maxVal) {
					return fmt.Errorf("field '%s' must be <= %d", structField.Name, maxVal)
				}
			case reflect.String:
				v := field.String()
				if minStr != "" && len(v) < minVal {
					return fmt.Errorf("field '%s' must be at least %d characters", structField.Name, minVal)
				}
				if maxStr != "" && len(v) > maxVal {
					return fmt.Errorf("field '%s' must be at most %d characters", structField.Name, maxVal)
				}
			}
		}
	}
	return nil
}

func (c *Ctx) Next() {
	c.index++
	if c.index < len(c.handlers) {
		c.handlers[c.index](c)
	}
}

func (c *Ctx) Abort() {
	c.index = len(c.handlers)
}

func (c *Ctx) AbortWithStatus(code int) {
	c.Status(code)
	c.Abort()
	c.Writer.WriteHeader(code)
	c.written = true
}

func (c *Ctx) IsAborted() bool {
	return c.index >= len(c.handlers)
}

func (c *Ctx) ClientIP() string {
	forwarded := c.Request.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	realIP := c.Request.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}
	idx := strings.LastIndex(c.Request.RemoteAddr, ":")
	if idx == -1 {
		return c.Request.RemoteAddr
	}
	return c.Request.RemoteAddr[:idx]
}

func (c *Ctx) StatusCode() int {
	return c.status
}

func (c *Ctx) Method() string {
	return c.Request.Method
}

func (c *Ctx) Path() string {
	return c.Request.URL.Path
}

type HandlerFunc func(*Ctx)

func setField(field reflect.Value, value string) error {
	if !field.CanSet() {
		return nil
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(b)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(f)
	default:
		return fmt.Errorf("unsupported type: %s", field.Kind())
	}
	return nil
}

func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}
