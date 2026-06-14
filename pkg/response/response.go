package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

// OK sends a 200 JSON response.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

// Created sends a 201 JSON response.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, data)
}

// BadRequest sends a 400 error response.
func BadRequest(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, ErrorResponse{Error: message})
}

// Unauthorized sends a 401 error response.
func Unauthorized(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: message})
}

// Forbidden sends a 403 error response.
func Forbidden(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusForbidden, ErrorResponse{Error: message})
}

// NotFound sends a 404 error response.
func NotFound(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusNotFound, ErrorResponse{Error: message})
}

// Conflict sends a 409 error response.
func Conflict(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusConflict, ErrorResponse{Error: message})
}

// ServiceUnavailable sends a 503 error response with an optional code.
func ServiceUnavailable(c *gin.Context, message string, code string) {
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, ErrorResponse{
		Error: message,
		Code:  code,
	})
}

// BadGateway sends a 502 error response with an optional code.
func BadGateway(c *gin.Context, message string, code string) {
	c.AbortWithStatusJSON(http.StatusBadGateway, ErrorResponse{
		Error: message,
		Code:  code,
	})
}

// InternalError sends a 500 error response.
func InternalError(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{Error: message})
}
