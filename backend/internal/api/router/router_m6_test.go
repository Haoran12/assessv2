package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestM6ImportTemplatePreviewAndConfirm(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	createOrganization(t, db, "M6 Import Company", "company", "active", nil)
	yearID := createAssessmentYearForTest(t, engine, rootToken, 2101)
	activateYearAndPeriodForTest(t, engine, db, rootToken, yearID, "Q1")
	objectIDs := listAssessmentObjectIDsForYear(t, engine, rootToken, yearID)
	if len(objectIDs) < 2 {
		t.Fatalf("expected at least two assessment objects for import, got=%d", len(objectIDs))
	}

	ruleID := createM4Rule(t, engine, rootToken, yearID, "Q1", "team", "subsidiary_company", []map[string]any{
		{
			"moduleCode": "direct",
			"moduleKey":  "m6_import_direct",
			"moduleName": "M6 Import Direct",
			"weight":     1.0,
			"maxScore":   100,
			"isActive":   true,
		},
	})
	moduleID := mustModuleIDByRuleAndCode(t, db, ruleID, "direct")

	templateReq := httptest.NewRequest(http.MethodGet, "/api/scores/import/templates/direct-score", nil)
	templateReq.Header.Set("Authorization", "Bearer "+rootToken)
	templateResp := httptest.NewRecorder()
	engine.ServeHTTP(templateResp, templateReq)
	if templateResp.Code != http.StatusOK {
		t.Fatalf("expected template download status=200, got=%d body=%s", templateResp.Code, templateResp.Body.String())
	}
	if len(templateResp.Body.Bytes()) == 0 {
		t.Fatalf("expected non-empty template file")
	}

	importFile := buildM6ImportWorkbook(t, objectIDs[:2], []float64{87.5, 91.2})
	previewBody := &bytes.Buffer{}
	writer := multipart.NewWriter(previewBody)
	_ = writer.WriteField("yearId", strconv.FormatUint(uint64(yearID), 10))
	_ = writer.WriteField("periodCode", "Q1")
	_ = writer.WriteField("moduleId", strconv.FormatUint(uint64(moduleID), 10))
	_ = writer.WriteField("overwrite", "false")
	filePart, err := writer.CreateFormFile("file", "direct-import.xlsx")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := filePart.Write(importFile); err != nil {
		t.Fatalf("failed to write import file body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	previewReq := httptest.NewRequest(http.MethodPost, "/api/scores/import/direct/preview", previewBody)
	previewReq.Header.Set("Authorization", "Bearer "+rootToken)
	previewReq.Header.Set("Content-Type", writer.FormDataContentType())
	previewResp := httptest.NewRecorder()
	engine.ServeHTTP(previewResp, previewReq)
	if previewResp.Code != http.StatusOK {
		t.Fatalf("expected import preview status=200, got=%d body=%s", previewResp.Code, previewResp.Body.String())
	}

	var previewEnvelope apiEnvelope
	if err := json.Unmarshal(previewResp.Body.Bytes(), &previewEnvelope); err != nil {
		t.Fatalf("failed to parse preview response: %v", err)
	}
	var previewData struct {
		ValidRows int `json:"validRows"`
		Rows      []struct {
			ObjectID uint    `json:"objectId"`
			Score    float64 `json:"score"`
			Remark   string  `json:"remark"`
			Status   string  `json:"status"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(previewEnvelope.Data, &previewData); err != nil {
		t.Fatalf("failed to parse preview payload: %v", err)
	}
	if previewData.ValidRows != 2 || len(previewData.Rows) != 2 {
		t.Fatalf("expected preview validRows=2 rows=2, got validRows=%d rows=%d", previewData.ValidRows, len(previewData.Rows))
	}

	confirmRows := make([]map[string]any, 0, len(previewData.Rows))
	for _, item := range previewData.Rows {
		if item.Status == "error" {
			continue
		}
		confirmRows = append(confirmRows, map[string]any{
			"objectId": item.ObjectID,
			"score":    item.Score,
			"remark":   item.Remark,
		})
	}
	confirmBody, _ := json.Marshal(map[string]any{
		"yearId":     yearID,
		"periodCode": "Q1",
		"moduleId":   moduleID,
		"overwrite":  false,
		"rows":       confirmRows,
	})
	confirmReq := httptest.NewRequest(http.MethodPost, "/api/scores/import/direct/confirm", bytes.NewReader(confirmBody))
	confirmReq.Header.Set("Authorization", "Bearer "+rootToken)
	confirmReq.Header.Set("Content-Type", "application/json")
	confirmResp := httptest.NewRecorder()
	engine.ServeHTTP(confirmResp, confirmReq)
	if confirmResp.Code != http.StatusOK {
		t.Fatalf("expected import confirm status=200, got=%d body=%s", confirmResp.Code, confirmResp.Body.String())
	}

	var confirmEnvelope apiEnvelope
	if err := json.Unmarshal(confirmResp.Body.Bytes(), &confirmEnvelope); err != nil {
		t.Fatalf("failed to parse confirm response: %v", err)
	}
	var confirmData struct {
		Created  int `json:"created"`
		Imported int `json:"imported"`
	}
	if err := json.Unmarshal(confirmEnvelope.Data, &confirmData); err != nil {
		t.Fatalf("failed to parse confirm payload: %v", err)
	}
	if confirmData.Created != 2 || confirmData.Imported != 2 {
		t.Fatalf("expected confirm result created=2 imported=2, got created=%d imported=%d", confirmData.Created, confirmData.Imported)
	}
}

func TestM6ExportWorkbook(t *testing.T) {
	engine, db := setupTestServer(t)
	rootToken, _ := loginAndReadData(t, engine, "root", testDefaultPassword)

	company := createOrganization(t, db, "M6 Export Company", "company", "active", nil)
	yearID := createAssessmentYearForTest(t, engine, rootToken, 2102)
	activateYearAndPeriodForTest(t, engine, db, rootToken, yearID, "Q1")
	objectID := mustAssessmentObjectIDByTarget(t, db, yearID, "organization", company.ID)

	ruleID := createM4Rule(t, engine, rootToken, yearID, "Q1", "team", "subsidiary_company", []map[string]any{
		{
			"moduleCode": "direct",
			"moduleKey":  "m6_export_direct",
			"moduleName": "M6 Export Direct",
			"weight":     1.0,
			"maxScore":   100,
			"isActive":   true,
		},
	})
	moduleID := mustModuleIDByRuleAndCode(t, db, ruleID, "direct")
	_ = createDirectScoreForTest(t, engine, rootToken, map[string]any{
		"yearId":     yearID,
		"periodCode": "Q1",
		"moduleId":   moduleID,
		"objectId":   objectID,
		"score":      93.25,
	})

	exportReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/reports/export?yearId=%d&periodCode=Q1", yearID), nil)
	exportReq.Header.Set("Authorization", "Bearer "+rootToken)
	exportResp := httptest.NewRecorder()
	engine.ServeHTTP(exportResp, exportReq)
	if exportResp.Code != http.StatusOK {
		t.Fatalf("expected export workbook status=200, got=%d body=%s", exportResp.Code, exportResp.Body.String())
	}
	if len(exportResp.Body.Bytes()) == 0 {
		t.Fatalf("expected non-empty export file content")
	}

	workbook, err := excelize.OpenReader(bytes.NewReader(exportResp.Body.Bytes()))
	if err != nil {
		t.Fatalf("failed to open exported workbook: %v", err)
	}
	defer func() {
		_ = workbook.Close()
	}()

	sheetSet := map[string]struct{}{}
	for _, item := range workbook.GetSheetList() {
		sheetSet[item] = struct{}{}
	}
	expectedSheets := []string{"结果汇总", "考核明细", "投票统计", "组织-单位", "组织-部门", "组织-人员"}
	for _, sheet := range expectedSheets {
		if _, exists := sheetSet[sheet]; !exists {
			t.Fatalf("expected export workbook to contain sheet %s, actual=%v", sheet, workbook.GetSheetList())
		}
	}

	summaryRows, err := workbook.GetRows("结果汇总")
	if err != nil {
		t.Fatalf("failed to read summary sheet: %v", err)
	}
	if len(summaryRows) < 2 {
		t.Fatalf("expected summary sheet to have data rows, got rowCount=%d", len(summaryRows))
	}
}

func buildM6ImportWorkbook(t *testing.T, objectIDs []uint, scores []float64) []byte {
	t.Helper()
	if len(objectIDs) != len(scores) {
		t.Fatalf("objectIDs and scores length mismatch")
	}

	file := excelize.NewFile()
	sheet := "Sheet1"
	_ = file.SetCellValue(sheet, "A1", "考核对象ID*")
	_ = file.SetCellValue(sheet, "B1", "考核对象名称(参考)")
	_ = file.SetCellValue(sheet, "C1", "分数*")
	_ = file.SetCellValue(sheet, "D1", "备注")

	for index := range objectIDs {
		rowNo := index + 2
		_ = file.SetCellValue(sheet, fmt.Sprintf("A%d", rowNo), objectIDs[index])
		_ = file.SetCellValue(sheet, fmt.Sprintf("B%d", rowNo), fmt.Sprintf("对象-%d", objectIDs[index]))
		_ = file.SetCellValue(sheet, fmt.Sprintf("C%d", rowNo), scores[index])
		_ = file.SetCellValue(sheet, fmt.Sprintf("D%d", rowNo), "M6批量导入测试")
	}

	buffer, err := file.WriteToBuffer()
	if err != nil {
		t.Fatalf("failed to build import workbook: %v", err)
	}
	_ = file.Close()
	return buffer.Bytes()
}
