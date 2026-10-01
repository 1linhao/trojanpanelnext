package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strconv"
	"strings"
	"trojan-panel/dao"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
	"trojan-panel/service"
	"trojan-panel/util"
)

// CompleteHostRemoval authenticates the host's callback with the random receipt
// stored in the deletion transaction. The receipt is sent only in the body.
func CompleteHostRemoval(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var request struct {
		NodeID  uint   `json:"nodeId"`
		Receipt string `json:"receipt"`
	}
	err := decoder.Decode(&request)
	decoded, decodeErr := hex.DecodeString(request.Receipt)
	if err != nil || request.NodeID == 0 || decodeErr != nil || len(decoded) != 32 || request.Receipt != hex.EncodeToString(decoded) {
		c.Status(http.StatusBadRequest)
		return
	}
	if err = decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		c.Status(http.StatusBadRequest)
		return
	}
	if err = dao.DeleteRemovalCleanup(request.NodeID, request.Receipt); errors.Is(err, dao.ErrInvalidRemovalReceipt) {
		c.Status(http.StatusConflict)
		return
	} else if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusNoContent)
}

func SelectNodeServerById(c *gin.Context) {
	var nodeServerRequireIdDto dto.RequiredIdDto
	_ = c.ShouldBindQuery(&nodeServerRequireIdDto)
	if err := validate.Struct(&nodeServerRequireIdDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	nodeServer, err := service.SelectNodeServerById(nodeServerRequireIdDto.Id)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	nodeServerOneVo := vo.NodeServerOneVo{
		Id:                *nodeServer.Id,
		Name:              *nodeServer.Name,
		Ip:                *nodeServer.Ip,
		GrpcPort:          *nodeServer.GrpcPort,
		GrpcTLSMode:       *nodeServer.GrpcTLSMode,
		GrpcTLSServerName: *nodeServer.GrpcTLSServerName,
		TrafficPeriod:     *nodeServer.TrafficPeriod, TrafficLimitMode: *nodeServer.TrafficLimitMode,
		TrafficTotalLimit: *nodeServer.TrafficTotalLimit, TrafficUploadLimit: *nodeServer.TrafficUploadLimit,
		TrafficDownloadLimit: *nodeServer.TrafficDownloadLimit,
		CreateTime:           *nodeServer.CreateTime,
	}
	statuses, err := service.ServerTrafficStatuses([]uint{nodeServerOneVo.Id})
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	nodeServerOneVo.TrafficStatus = statuses[nodeServerOneVo.Id]
	vo.Success(nodeServerOneVo, c)
}

func CreateNodeServer(c *gin.Context) {
	var nodeServerCreateDto dto.NodeServerCreateDto
	_ = c.ShouldBindJSON(&nodeServerCreateDto)
	if err := validate.Struct(&nodeServerCreateDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	nodeServer := model.NodeServer{
		Name:              nodeServerCreateDto.Name,
		Ip:                nodeServerCreateDto.Ip,
		GrpcPort:          nodeServerCreateDto.GrpcPort,
		GrpcTLSServerName: nodeServerCreateDto.GrpcTLSServerName,
		TrafficPeriod:     nodeServerCreateDto.TrafficPeriod, TrafficLimitMode: nodeServerCreateDto.TrafficLimitMode,
		TrafficTotalLimit: nodeServerCreateDto.TrafficTotalLimit, TrafficUploadLimit: nodeServerCreateDto.TrafficUploadLimit,
		TrafficDownloadLimit: nodeServerCreateDto.TrafficDownloadLimit,
	}
	if err := service.CreateNodeServer(&nodeServer); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func SelectNodeServerPage(c *gin.Context) {
	var nodeServerPageDto dto.NodeServerPageDto
	_ = c.ShouldBindQuery(&nodeServerPageDto)
	if err := validate.Struct(&nodeServerPageDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	nodeServerPageVo, err := service.SelectNodeServerPage(nodeServerPageDto.Name, nodeServerPageDto.Ip, nodeServerPageDto.PageNum, nodeServerPageDto.PageSize, c)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nodeServerPageVo, c)
}

// DeleteNodeServerById deletes Web records without contacting the target host.
func DeleteNodeServerById(c *gin.Context) {
	var request dto.RequiredIdDto
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if request.Id == nil || *request.Id == 0 {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := validate.Struct(&request); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := service.DeleteNodeServerById(request.Id); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func UninstallNodeServerById(c *gin.Context) {
	var nodeServerRequireIdDto dto.NodeServerUninstallDto
	if err := c.ShouldBindJSON(&nodeServerRequireIdDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := validate.Struct(&nodeServerRequireIdDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	result, err := service.UninstallNodeServerById(nodeServerRequireIdDto.Id, nodeServerRequireIdDto.Purge)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(result, c)
}

func ResetNodeServerTraffic(c *gin.Context) {
	var requiredID dto.RequiredIdDto
	_ = c.ShouldBindJSON(&requiredID)
	if err := validate.Struct(&requiredID); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	result, err := service.ResetNodeServerTraffic(requiredID.Id)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(result, c)
}

func UpdateNodeServerById(c *gin.Context) {
	var nodeServerUpdateDto dto.NodeServerUpdateDto
	_ = c.ShouldBindJSON(&nodeServerUpdateDto)
	if err := validate.Struct(&nodeServerUpdateDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	if err := service.UpdateNodeServerById(&nodeServerUpdateDto); err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nil, c)
}

func SelectNodeServerList(c *gin.Context) {
	var nodeServerDto dto.NodeServerDto
	_ = c.ShouldBindQuery(&nodeServerDto)
	if err := validate.Struct(&nodeServerDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	nodeServerListVos, err := service.SelectNodeServerList(&nodeServerDto)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	vo.Success(nodeServerListVos, c)
}

func GetNodeServerInfo(c *gin.Context) {
	var requiredIdDto dto.RequiredIdDto
	_ = c.ShouldBindQuery(&requiredIdDto)
	if err := validate.Struct(&requiredIdDto); err != nil {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	nodeServerInfo, err := service.GetNodeServerInfo(util.GetToken(c), requiredIdDto.Id)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	nodeServerInfoVo := vo.NodeServerInfoVo{
		CpuUsed:  nodeServerInfo.CpuUsed,
		MemUsed:  nodeServerInfo.MemUsed,
		DiskUsed: nodeServerInfo.DiskUsed,
	}
	vo.Success(nodeServerInfoVo, c)
}

// ExportNodeServer 导出服务器
func ExportNodeServer(c *gin.Context) {
	accountVo := service.GetCurrentAccount(c)
	if err := service.ExportNodeServer(accountVo.Id, accountVo.Username); err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	vo.Success(nil, c)
}

// ImportNodeServer 导入服务器
func ImportNodeServer(c *gin.Context) {
	coverStr, b := c.GetPostForm("cover")
	if !b {
		vo.Fail(constant.ValidateFailed, c)
		return
	}
	cover, err := strconv.ParseUint(coverStr, 10, 32)
	file, err := c.FormFile("file")
	if err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	// 文件大小 10MB
	if file.Size > 1024*1024*10 {
		vo.Fail(constant.FileSizeTooBig, c)
		return
	}
	// 文件后缀.json
	if !strings.HasSuffix(file.Filename, ".json") {
		vo.Fail(constant.FileFormatError, c)
		return
	}
	account := service.GetCurrentAccount(c)
	if err := service.ImportNodeServer(uint(cover), file, account.Id, account.Username); err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	vo.Success(nil, c)
}
