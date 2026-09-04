package util

import (
	"bufio"
	"encoding/csv"
	"github.com/sirupsen/logrus"
	"os"
)

func ExportCsv(filePath string, data [][]string) error {
	file, err := os.Create(filePath)
	if err != nil {
		logrus.Errorf("create csv file err filePath: %s err: %v", filePath, err)
		return err
	}
	defer file.Close()

	// 创建带有指定编码格式的 bufio.Writer
	writer := bufio.NewWriter(file)
	_, err = writer.WriteString("\xEF\xBB\xBF")
	if err != nil {
		logrus.Errorf("set csv file UTF-8 err filePath: %s err: %v", filePath, err)
		return err
	}
	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()
	if err = csvWriter.WriteAll(data); err != nil {
		logrus.Errorf("writeAll csv file err fileName: %s err: %v", filePath, err)
		return err
	}
	return nil
}
