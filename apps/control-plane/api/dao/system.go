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
		"select id,account_config,email_config,template_config,template_library from `system` where name = {{name}}",
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
	// Merge just the template names sent by the caller. Older settings forms
	// omit those fields and the library, and can run alongside catalog edits.
	_, err := db.Exec("UPDATE `system` SET "+
		"account_config=COALESCE(?,account_config), email_config=COALESCE(?,email_config), "+
		"template_config=JSON_MERGE_PATCH(template_config,COALESCE(?, '{}')), "+
		"template_library=COALESCE(?,template_library) WHERE id=?",
		system.AccountConfig, system.EmailConfig, system.TemplateConfig, system.TemplateLibrary, system.Id)
	if err != nil {
		logrus.Errorln(err.Error())
		return errors.New(constant.SysError)
	}
	return nil
}
