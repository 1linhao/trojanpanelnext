package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
	"trojan-panel/service"
)

func NodeServerDeployment(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	id, err := strconv.ParseUint(c.Query("id"), 10, 64)
	if err != nil || id == 0 {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	metadata, err := service.NodeServerDeployment(uint(id))
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(metadata, c)
}

func DownloadNodeDeployment(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("Pragma", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var request dto.NodeDeploymentDownloadDto
	if err := decoder.Decode(&request); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	archive, err := service.DownloadNodeDeployment(request)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="tpnext-node-%d.tar.gz"`, request.Id))
	c.Data(http.StatusOK, "application/gzip", archive)
}
