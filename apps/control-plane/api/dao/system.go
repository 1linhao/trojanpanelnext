package dao

import (
	"errors"
	"github.com/didi/gendry/builder"
	"github.com/didi/gendry/scanner"
	"github.com/sirupsen/logrus"
	"trojan-panel/model"
	"trojan-panel/model/constant"
)

func SelectSystemByName(name *string) (*model.System, error) {
	var system model.System
	buildSelect, values, err := builder.NamedQuery(
		"select id,account_config,email_config,template_config from `system` where name = {{name}}",
		map[string]interface{}{"name": *name})
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	rows, err := db.Query(buildSelect, values...)
	if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}
	defer rows.Close()

	if err = scanner.Scan(rows, &system); err == scanner.ErrEmptyResult {
		return nil, errors.New(constant.SystemNotExist)
	} else if err != nil {
		logrus.Errorln(err.Error())
		return nil, errors.New(constant.SysError)
	}

	return &system, nil
}

func UpdateSystemById(system *model.System) error {
	where := map[string]interface{}{"id": *system.Id}
	update := map[string]interface{}{}
	if system.AccountConfig != nil {
		update["account_config"] = *system.AccountConfig
	}
	if system.EmailConfig != nil {
		update["email_config"] = *system.EmailConfig
	}
	if system.TemplateConfig != nil {
		update["template_config"] = *system.TemplateConfig
	}
	if len(update) > 0 {
		buildUpdate, values, err := builder.BuildUpdate("`system`", where, update)
		if err != nil {
			logrus.Errorln(err.Error())
			return errors.New(constant.SysError)
		}
		if _, err = db.Exec(buildUpdate, values...); err != nil {
			logrus.Errorln(err.Error())
			return errors.New(constant.SysError)
		}
	}
	return nil
}
