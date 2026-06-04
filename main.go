package main

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/12ain/rmb-uppercase-converter/internal/converter"
	"github.com/gin-gonic/gin"
)

//go:embed static
var staticFS embed.FS

func buildRouter() *gin.Engine {
	r := gin.Default()

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}

	serveFile := func(name, contentType string) gin.HandlerFunc {
		return func(c *gin.Context) {
			data, err := staticFS.ReadFile("static/" + name)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			c.Data(http.StatusOK, contentType, data)
		}
	}

	r.GET("/", serveFile("index.html", "text/html; charset=utf-8"))
	r.GET("/docs", serveFile("docs.html", "text/html; charset=utf-8"))
	r.GET("/docs/spec", serveFile("swagger.html", "text/html; charset=utf-8"))
	r.GET("/openapi.json", serveFile("openapi.json", "application/json"))
	r.StaticFS("/static", http.FS(sub))

	r.POST("/api/convert", handleConvert)
	r.POST("/api/convert/reverse", handleReverse)
	r.POST("/api/convert/verify", handleVerify)
	r.POST("/api/convert/batch", handleBatch)

	return r
}

func main() {
	buildRouter().Run(":8080")
}

func handleConvert(c *gin.Context) {
	var req struct {
		Amount string `json:"amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": string(converter.ErrInvalidFormat), "message": "invalid JSON"})
		return
	}
	chinese, err := converter.Forward(req.Amount)
	if err != nil {
		writeConverterError(c, err)
		return
	}
	c.JSON(200, gin.H{"chinese": chinese})
}

func handleReverse(c *gin.Context) {
	var req struct {
		Chinese string `json:"chinese"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": string(converter.ErrUnparsableChinese), "message": "invalid JSON"})
		return
	}
	amount, err := converter.Reverse(req.Chinese)
	if err != nil {
		writeConverterError(c, err)
		return
	}
	c.JSON(200, gin.H{"amount": amount})
}

func handleVerify(c *gin.Context) {
	var req struct {
		Amount  string `json:"amount"`
		Chinese string `json:"chinese"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": string(converter.ErrInvalidFormat), "message": "invalid JSON"})
		return
	}
	result, err := converter.Verify(req.Amount, req.Chinese)
	if err != nil {
		writeConverterError(c, err)
		return
	}
	c.JSON(200, result)
}

func handleBatch(c *gin.Context) {
	var req struct {
		Amounts []string `json:"amounts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": string(converter.ErrInvalidFormat), "message": "invalid JSON"})
		return
	}
	results, err := converter.Batch(req.Amounts)
	if err != nil {
		writeConverterError(c, err)
		return
	}
	c.JSON(200, gin.H{"results": results})
}

func writeConverterError(c *gin.Context, err error) {
	if ce, ok := err.(*converter.ConverterError); ok {
		status := 400
		if ce.Code == converter.ErrBatchTooLarge {
			status = 413
		}
		resp := gin.H{"error": string(ce.Code), "message": ce.Message}
		if ce.At > 0 {
			resp["at"] = ce.At
		}
		c.JSON(status, resp)
		return
	}
	c.JSON(500, gin.H{"error": "internal", "message": err.Error()})
}
