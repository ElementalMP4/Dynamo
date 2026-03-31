package util

import (
	"net/http"
	
	"github.com/ElementalMP4/loggerino/log"
	"github.com/gin-gonic/gin"
)

func ErrorResponse(c *gin.Context, httpStatus int, source string, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	log.Errorf(source, "HTTP Error: %v", err)
}