package tests

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type us010APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func newUS010Context(t *testing.T, method string, target string, body any, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var requestBody *bytes.Reader
	if body != nil {
		payload, err := common.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		requestBody = bytes.NewReader(payload)
	} else {
		requestBody = bytes.NewReader(nil)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, requestBody)
	if body != nil {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	ctx.Set("id", userID)
	return ctx, recorder
}

func decodeUS010Response(t *testing.T, recorder *httptest.ResponseRecorder) us010APIResponse {
	t.Helper()
	var resp us010APIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode api response: %v", err)
	}
	return resp
}

func TestUS010_CreateAPIKey(t *testing.T) {
	user := &model.User{
		Username: "us010-owner",
		Email:    "us010-owner@example.com",
		AffCode:  nextAffCode("US010"),
		Status:   1,
	}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	body := map[string]any{
		"name":                 "us010-key",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}
	ctx, recorder := newUS010Context(t, http.MethodPost, "/api/token/", body, int(user.Id))
	controller.AddToken(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", recorder.Code)
	}
	resp := decodeUS010Response(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}

	var token model.Token
	if err := testDB.Where("user_id = ? AND name = ?", user.Id, "us010-key").First(&token).Error; err != nil {
		t.Fatalf("expected token persisted in DB: %v", err)
	}
	if len(token.Key) != 48 {
		t.Fatalf("expected generated key length 48, got %d", len(token.Key))
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("create response leaked full key: %s", recorder.Body.String())
	}
}

func TestUS010_CreateAPIKeyRejectsTooLongName(t *testing.T) {
	user := &model.User{
		Username: "us010-invalid-owner",
		Email:    "us010-invalid-owner@example.com",
		AffCode:  nextAffCode("US010N"),
		Status:   1,
	}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	longName := strings.Repeat("a", 51)
	body := map[string]any{
		"name":                 longName,
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}
	ctx, recorder := newUS010Context(t, http.MethodPost, "/api/token/", body, int(user.Id))
	controller.AddToken(ctx)

	resp := decodeUS010Response(t, recorder)
	if resp.Success {
		t.Fatalf("expected success=false for too long name")
	}

	var count int64
	if err := testDB.Model(&model.Token{}).Where("user_id = ? AND name = ?", user.Id, longName).Count(&count).Error; err != nil {
		t.Fatalf("failed to count tokens after rejection: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no token persisted for invalid name, got %d", count)
	}
}

func TestUS010_GetTokenKeyRequiresOwnership(t *testing.T) {
	owner := &model.User{
		Username: "us010-key-owner",
		Email:    "us010-key-owner@example.com",
		AffCode:  nextAffCode("US010O"),
		Status:   1,
	}
	other := &model.User{
		Username: "us010-other",
		Email:    "us010-other@example.com",
		AffCode:  nextAffCode("US010P"),
		Status:   1,
	}
	if err := testDB.Create(owner).Error; err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}
	if err := testDB.Create(other).Error; err != nil {
		t.Fatalf("failed to create other user: %v", err)
	}

	token := &model.Token{
		UserId:         int(owner.Id),
		Name:           "us010-owned-key",
		Key:            "owner1234token5678owner1234token5678owner1234tok",
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	if err := testDB.Create(token).Error; err != nil {
		t.Fatalf("failed to create owned token: %v", err)
	}

	ownerCtx, ownerRecorder := newUS010Context(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, int(owner.Id))
	ownerCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	controller.GetTokenKey(ownerCtx)

	ownerResp := decodeUS010Response(t, ownerRecorder)
	if !ownerResp.Success {
		t.Fatalf("expected owner fetch key success, got message: %s", ownerResp.Message)
	}
	if !strings.Contains(ownerRecorder.Body.String(), token.Key) {
		t.Fatalf("expected owner response to contain full key")
	}

	otherCtx, otherRecorder := newUS010Context(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, int(other.Id))
	otherCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	controller.GetTokenKey(otherCtx)

	otherResp := decodeUS010Response(t, otherRecorder)
	if otherResp.Success {
		t.Fatalf("expected non-owner fetch key to fail")
	}
	if strings.Contains(otherRecorder.Body.String(), token.Key) {
		t.Fatalf("non-owner response leaked full key: %s", otherRecorder.Body.String())
	}
}

func TestUS011_ListAPIKeysMasksAndScopesToOwner(t *testing.T) {
	owner := &model.User{
		Username: "us011-owner",
		Email:    "us011-owner@example.com",
		AffCode:  nextAffCode("US011"),
		Status:   1,
	}
	other := &model.User{
		Username: "us011-other",
		Email:    "us011-other@example.com",
		AffCode:  nextAffCode("US011X"),
		Status:   1,
	}
	if err := testDB.Create(owner).Error; err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}
	if err := testDB.Create(other).Error; err != nil {
		t.Fatalf("failed to create other user: %v", err)
	}

	ownerToken := &model.Token{
		UserId:         int(owner.Id),
		Name:           "us011-owned-key",
		Key:            "abcd1234ownerkey5678abcd1234ownerkey5678abcd1234",
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	otherToken := &model.Token{
		UserId:         int(other.Id),
		Name:           "us011-other-key",
		Key:            "wxyz1234otherkey5678wxyz1234otherkey5678wxyz1234",
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	if err := testDB.Create(ownerToken).Error; err != nil {
		t.Fatalf("failed to create owner token: %v", err)
	}
	if err := testDB.Create(otherToken).Error; err != nil {
		t.Fatalf("failed to create other token: %v", err)
	}

	ctx, recorder := newUS010Context(t, http.MethodGet, "/api/token/?p=1&size=10", nil, int(owner.Id))
	controller.GetAllTokens(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", recorder.Code)
	}
	resp := decodeUS010Response(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if strings.Contains(recorder.Body.String(), ownerToken.Key) {
		t.Fatalf("list response leaked full owner key: %s", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), otherToken.Name) {
		t.Fatalf("list response leaked other user's token: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), ownerToken.GetMaskedKey()) {
		t.Fatalf("expected masked owner key in response, got: %s", recorder.Body.String())
	}
}

func TestUS012_RevokeAPIKeyDeletesAndDeniesRepeatDelete(t *testing.T) {
	owner := &model.User{
		Username: "us012-owner",
		Email:    "us012-owner@example.com",
		AffCode:  nextAffCode("US012"),
		Status:   1,
	}
	if err := testDB.Create(owner).Error; err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	token := &model.Token{
		UserId:         int(owner.Id),
		Name:           "us012-revoke-key",
		Key:            "revoke1234token5678revoke1234token5678revoke1234to",
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	if err := testDB.Create(token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	deleteCtx, deleteRecorder := newUS010Context(t, http.MethodDelete, "/api/token/"+strconv.Itoa(token.Id), nil, int(owner.Id))
	deleteCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	controller.DeleteToken(deleteCtx)

	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for first delete, got %d", deleteRecorder.Code)
	}
	deleteResp := decodeUS010Response(t, deleteRecorder)
	if !deleteResp.Success {
		t.Fatalf("expected first delete success, got message: %s", deleteResp.Message)
	}

	var count int64
	if err := testDB.Model(&model.Token{}).Where("id = ? AND user_id = ?", token.Id, owner.Id).Count(&count).Error; err != nil {
		t.Fatalf("failed to count token after delete: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected token removed after delete, got %d", count)
	}

	repeatCtx, repeatRecorder := newUS010Context(t, http.MethodDelete, "/api/token/"+strconv.Itoa(token.Id), nil, int(owner.Id))
	repeatCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	controller.DeleteToken(repeatCtx)

	repeatResp := decodeUS010Response(t, repeatRecorder)
	if repeatResp.Success {
		t.Fatalf("expected repeat delete to fail for missing token")
	}
}

func TestUS013_SetAPIKeyLimitPersistsAndRejectsNegativeQuota(t *testing.T) {
	owner := &model.User{
		Username: "us013-owner",
		Email:    "us013-owner@example.com",
		AffCode:  nextAffCode("US013"),
		Status:   1,
	}
	if err := testDB.Create(owner).Error; err != nil {
		t.Fatalf("failed to create owner: %v", err)
	}

	token := &model.Token{
		UserId:         int(owner.Id),
		Name:           "us013-limit-key",
		Key:            "limit1234token5678limit1234token5678limit1234token5",
		Status:         common.TokenStatusEnabled,
		CreatedTime:    common.GetTimestamp(),
		AccessedTime:   common.GetTimestamp(),
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	if err := testDB.Create(token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	validBody := map[string]any{
		"id":                   token.Id,
		"name":                 token.Name,
		"expired_time":         -1,
		"remain_quota":         37,
		"unlimited_quota":      false,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}
	updateCtx, updateRecorder := newUS010Context(t, http.MethodPut, "/api/token/", validBody, int(owner.Id))
	controller.UpdateToken(updateCtx)

	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for quota update, got %d", updateRecorder.Code)
	}
	updateResp := decodeUS010Response(t, updateRecorder)
	if !updateResp.Success {
		t.Fatalf("expected update success, got message: %s", updateResp.Message)
	}
	if strings.Contains(updateRecorder.Body.String(), token.Key) {
		t.Fatalf("update response leaked full key: %s", updateRecorder.Body.String())
	}

	var persisted model.Token
	if err := testDB.Where("id = ? AND user_id = ?", token.Id, owner.Id).First(&persisted).Error; err != nil {
		t.Fatalf("failed to load updated token: %v", err)
	}
	if persisted.RemainQuota != 37 {
		t.Fatalf("expected remain_quota=37, got %d", persisted.RemainQuota)
	}
	if persisted.UnlimitedQuota {
		t.Fatalf("expected unlimited_quota=false after limit set")
	}

	invalidBody := map[string]any{
		"id":                   token.Id,
		"name":                 token.Name,
		"expired_time":         -1,
		"remain_quota":         -1,
		"unlimited_quota":      false,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}
	invalidCtx, invalidRecorder := newUS010Context(t, http.MethodPut, "/api/token/", invalidBody, int(owner.Id))
	controller.UpdateToken(invalidCtx)

	invalidResp := decodeUS010Response(t, invalidRecorder)
	if invalidResp.Success {
		t.Fatalf("expected update failure for negative quota")
	}

	var afterInvalid model.Token
	if err := testDB.Where("id = ? AND user_id = ?", token.Id, owner.Id).First(&afterInvalid).Error; err != nil {
		t.Fatalf("failed to reload token after invalid update: %v", err)
	}
	if afterInvalid.RemainQuota != 37 {
		t.Fatalf("expected remain_quota unchanged at 37 after invalid update, got %d", afterInvalid.RemainQuota)
	}
}
