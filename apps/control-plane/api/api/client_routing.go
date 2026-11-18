package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"trojan-panel/dao"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
	"trojan-panel/service/clientrouting"
)

func routingFailure(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		vo.Fail("版本不存在", c)
	case errors.Is(err, dao.ErrActiveRouting):
		vo.Fail(err.Error(), c)
	default:
		logrus.WithError(err).Error("client routing")
		vo.Fail(constant.SysError, c)
	}
}

func ClientRouting(c *gin.Context) {
	if !requireCurrentSysAdmin(c) {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	if value, ok := c.GetQuery("id"); ok {
		id, err := strconv.ParseUint(value, 10, 32)
		if err != nil || id == 0 {
			vo.Fail(constant.ValidateFailed, c)
			return
		}
		source, err := dao.RoutingSource(uint(id))
		if err != nil {
			routingFailure(c, err)
			return
		}
		vo.Success(json.RawMessage(source), c)
		return
	}
	nodes, err := dao.RoutingNodes()
	if err != nil {
		routingFailure(c, err)
		return
	}
	releases, err := dao.RoutingHistory()
	if err != nil {
		routingFailure(c, err)
		return
	}
	vo.Success(gin.H{"nodes": nodes, "releases": releases}, c)
}

func PublishClientRouting(c *gin.Context) {
	if !requireCurrentSysAdmin(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	var p clientrouting.Profile
	if err := c.ShouldBindJSON(&p); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	nodes, err := dao.RoutingNodes()
	if err != nil {
		routingFailure(c, err)
		return
	}
	servers := make(map[uint]uint, len(nodes))
	for _, n := range nodes {
		servers[n.ID] = n.ServerID
	}
	snapshot, err := clientrouting.Compile(p, servers)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	source, err := json.Marshal(p)
	if err != nil {
		routingFailure(c, err)
		return
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		routingFailure(c, err)
		return
	}
	id, err := dao.PublishRouting(p.Name, source, data)
	if err != nil {
		routingFailure(c, err)
		return
	}
	vo.Success(id, c)
}

func ChangeClientRouting(remove bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCurrentSysAdmin(c) {
			return
		}
		var request struct {
			ID *uint `json:"id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&request); err != nil || (remove && *request.ID == 0) {
			vo.Fail(constant.ValidateFailed, c)
			return
		}
		if err := dao.ChangeRoutingRelease(*request.ID, remove); err != nil {
			routingFailure(c, err)
			return
		}
		vo.Success(nil, c)
	}
}
