package util

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func QueryInt(c *gin.Context, key string, def int) int {
	v := c.Query(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

func ClampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func ParseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
