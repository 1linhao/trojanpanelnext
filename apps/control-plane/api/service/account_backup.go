package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strings"
	"trojan-panel/dao"
	"trojan-panel/model"
	"trojan-panel/model/constant"
)

type AccountImportFailure struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

type AccountImportResult struct {
	Imported int                    `json:"imported"`
	Skipped  int                    `json:"skipped"`
	Failures []AccountImportFailure `json:"failures"`
}

func ImportAccount(cover uint, file *multipart.FileHeader) (*AccountImportResult, error) {
	rows, err := decodeImportRows[json.RawMessage](file)
	if err != nil {
		return nil, err
	}
	result := &AccountImportResult{Failures: make([]AccountImportFailure, 0)}
	for i, row := range rows {
		var account model.Account
		var reason string
		switch {
		case json.Unmarshal(row, &account) != nil:
			reason = "invalidRow"
		case account.Username == nil || strings.TrimSpace(*account.Username) == "":
			reason = "usernameRequired"
		case account.RoleId == nil:
			reason = "roleRequired"
		case *account.RoleId == constant.SYSADMIN:
			result.Skipped++
		default:
			err := dao.CreateOrUpdateAccount(account, cover)
			switch {
			case err == nil:
				result.Imported++
			case errors.Is(err, dao.ErrAccountImportSkipped):
				result.Skipped++
			default:
				reason = "writeFailed"
			}
		}
		if reason != "" {
			result.Failures = append(result.Failures, AccountImportFailure{Row: i + 1, Reason: reason})
		}
	}
	return result, nil
}

func decodeImportRows[T any](file *multipart.FileHeader) ([]T, error) {
	src, err := file.Open()
	if err != nil {
		return nil, errors.New(constant.FileUploadError)
	}
	defer src.Close()

	var rows []T
	decoder := json.NewDecoder(src)
	if err := decoder.Decode(&rows); err != nil {
		return nil, errors.New(constant.FileFormatError)
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return nil, errors.New(constant.FileFormatError)
	}
	if len(rows) == 0 {
		return nil, errors.New(constant.RowNotEnough)
	}
	return rows, nil
}

// importJSON applies valid rows in order and reports any failures after processing the file.
func importJSON[T any](file *multipart.FileHeader, apply func(T) error) error {
	rows, err := decodeImportRows[T](file)
	if err != nil {
		return err
	}

	var firstError error
	var failures int
	for i, row := range rows {
		if err := apply(row); err != nil {
			failures++
			if firstError == nil {
				firstError = fmt.Errorf("row %d: %w", i+1, err)
			}
		}
	}
	if failures > 0 {
		return fmt.Errorf("import failed for %d of %d rows (%w)", failures, len(rows), firstError)
	}
	return nil
}
