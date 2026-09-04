package router

import (
	"github.com/gin-gonic/gin"
	"trojan-panel/api"
)

func initAccountRouter(trojanApi *gin.RouterGroup) {
	account := trojanApi.Group("/account")
	{
		// 注销
		account.POST("/logout", api.Logout)
		// 查询单个账户
		account.GET("/selectAccountById", api.SelectAccountById)
		// 创建账户
		account.POST("/createAccount", api.CreateAccount)
		// 获取当前用户信息
		account.GET("/getAccountInfo", api.GetAccountInfo)
		// 分页查询账户
		account.GET("/selectAccountPage", api.SelectAccountPage)
		// 通过id删除账户
		account.POST("/deleteAccountById", api.DeleteAccountById)
		// 修改密码
		account.POST("/updateAccountPass", api.UpdateAccountPass)
		// 修改用户名或者邮箱
		account.POST("/updateAccountProperty", api.UpdateAccountProperty)
		// 修改账户
		account.POST("/updateAccountById", api.UpdateAccountById)
		// 获取客户端导出选项
		account.GET("/exportOptions", api.ExportOptions)
		// 获取当前用户的订阅地址
		account.GET("/exportSubscribe", api.ExportSubscribe)
		// 获取当前用户的订阅二维码
		account.GET("/exportQRCode", api.ExportQRCode)
		// 重设下载和上传流量
		account.POST("/resetAccountDownloadAndUpload", api.ResetAccountDownloadAndUpload)
		// 导出用户
		account.POST("/exportAccount", api.ExportAccount)
		// 导入用户
		account.POST("/importAccount", api.ImportAccount)
		// 批量生成账户
		account.POST("/createAccountBatch", api.CreateAccountBatch)
		// 导出未使用的账户
		account.POST("/exportAccountUnused", api.ExportAccountUnused)
	}
}
