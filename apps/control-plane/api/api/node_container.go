package api

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strconv"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
	"trojan-panel/service"
)

func NodeContainerInventory(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	query := c.Request.URL.Query()
	values := query["nodeServerId"]
	if len(query) != 1 || len(values) != 1 {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	id, err := strconv.ParseUint(values[0], 10, strconv.IntSize)
	if err != nil || id == 0 {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	inventory, err := service.GetNodeContainerInventory(c.Request.Context(), accountManagementToken(c), uint(id))
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(inventory, c)
}

func UpdateNodeContainer(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var request dto.NodeContainerRequestDto
	if err := decoder.Decode(&request); err != nil || request.NodeServerId == 0 {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	job, err := service.StartNodeContainerUpdate(c.Request.Context(), accountManagementToken(c), request.NodeServerId)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(job, c)
}
