package api

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
	"trojan-panel/dao"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
	"trojan-panel/service"
	"trojan-panel/util"

	"github.com/gin-gonic/gin"
)

// ExportAccount 导出用户
func ExportAccount(c *gin.Context) {
	accounts, err := dao.SelectAccountAll()
	if err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	downloadJSON(c, exportFilename("accountExport"), accounts)
}

// ImportAccount 导入用户
func ImportAccount(c *gin.Context) {
	cover, file := getImportFile(c)
	if file == nil {
		return
	}
	result, err := service.ImportAccount(cover, file)
	if err != nil {
		vo.Fail(err.Error(), c)
		return
	}
	c.Header("Cache-Control", "no-store")
	vo.Success(result, c)
}

func ExportAccountUnused(c *gin.Context) {
	accounts, err := dao.SelectAccountUnused()
	if err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	downloadJSON(c, exportFilename("accountUnusedExport"), accounts)
}

func DownloadAccountImportTemplate(c *gin.Context) {
	pass := util.Sha1String("example" + "example")
	downloadJSON(c, "AccountTemplate.json", []map[string]any{{
		"username": "example", "pass": pass, "hash": util.SHA224String(pass),
		"roleId": constant.USER, "email": "test@example.com",
		"expireTime": int64(4078656000000), "quota": -1,
		"deleted": 0, "download": 0, "upload": 0,
	}})
}

func exportFilename(prefix string) string {
	return prefix + "-" + time.Now().Format("20060102150405") + ".json"
}

func downloadJSON[T any](c *gin.Context, filename string, rows []T) {
	if rows == nil {
		rows = []T{}
	}
	data, err := json.MarshalIndent(rows, "", "    ")
	if err != nil {
		vo.Fail(constant.SysError, c)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/json", data)
}

func getImportFile(c *gin.Context) (uint, *multipart.FileHeader) {
	var cover uint
	switch c.PostForm("cover") {
	case "0":
	case "1":
		cover = 1
	default:
		vo.Fail(constant.ValidateFailed, c)
		return 0, nil
	}
	file, err := c.FormFile("file")
	if err != nil {
		vo.Fail(constant.FileUploadError, c)
		return 0, nil
	}
	if file.Size > 10*1024*1024 {
		vo.Fail(constant.FileSizeTooBig, c)
		return 0, nil
	}
	if !strings.HasSuffix(file.Filename, ".json") {
		vo.Fail(constant.FileFormatError, c)
		return 0, nil
	}
	return cover, file
}
