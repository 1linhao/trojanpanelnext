package util

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"os"
)

func ExportJson(filePath string, data any) error {
	file, err := os.Create(filePath)
	if err != nil {
		logrus.Errorf("ExportJson create json file err filePath: %s err: %v", filePath, err)
		return err
	}
	defer file.Close()

	jsonBytes, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		logrus.Errorf("ExportJson json Marshal err filePath: %s err: %v", filePath, err)
		return err
	}
	_, err = file.Write(jsonBytes)
	if err != nil {
		logrus.Errorf("ExportJson writer WriteString err filePath: %s err: %v", filePath, err)
		return err
	}
	return nil
}
