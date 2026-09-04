package dao

import (
	"errors"
	"trojan-panel/model/constant"

	sqladapter "github.com/Blank-Xu/sql-adapter"
	"github.com/casbin/casbin/v2"
	"github.com/sirupsen/logrus"
)

func Casbin() (*casbin.Enforcer, error) {
	a, err := sqladapter.NewAdapter(db, "mysql", "casbin_rule")
	if err != nil {
		logrus.Errorf("casbin initialization err: %v", err)
		return nil, errors.New(constant.SysError)
	}
	// 读取conf配置文件
	e, err := casbin.NewEnforcer(constant.RbacModelFilePath, a)
	if err != nil {
		logrus.Errorf("configuration file not found err: %v", err)
		return nil, errors.New(constant.SysError)
	}
	// 加载规则
	if err := e.LoadPolicy(); err != nil {
		logrus.Errorf("failed to load rules err: %v", err)
		return nil, errors.New(constant.SysError)
	}
	return e, nil
}
