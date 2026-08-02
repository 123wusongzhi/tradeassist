package imagetask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/safedownload"
	imgprov "github.com/trademind-ai/trademind/backend/internal/providers/image"
)

// persistProviderResult uploads bytes to configured storage; never returns provider temp URLs as final.
func (s *Service) persistProviderResult(ctx context.Context, task *ImageTask, res *imgprov.ImageResult, hints map[string]any) (finalURL string, fileID *uuid.UUID, storageKey string, err error) {
	if res == nil {
		return "", nil, "", fmt.Errorf("empty provider result")
	}
	if s.Files == nil {
		return "", nil, "", fmt.Errorf("files service unavailable")
	}

	data := res.RawPayload
	ct := strings.TrimSpace(res.PayloadContentType)
	if len(data) == 0 {
		u := strings.TrimSpace(res.PublicURL)
		if u == "" {
			return "", nil, "", fmt.Errorf("provider returned empty result")
		}
		data, ct, err = downloadResultBytes(ctx, u)
		if err != nil {
			return "", nil, "", err
		}
	}
	if ct == "" {
		ct = "image/webp"
	}
	if !strings.HasPrefix(strings.ToLower(ct), "image/") {
		ct = "image/webp"
	}

	objKey := BuildAIImageObjectKey(task.ProductID, task.TaskType)
	origName := fmt.Sprintf("%s-%s.webp", task.TaskType, task.ID.String())

	opts := files.SaveProcessedOpts{
		TenantID:     s.persistTenantID(ctx, task),
		OriginalName: origName,
		ObjectKey:    objKey,
		Data:         data,
		ContentType:  ct,
		CreatedBy:    task.CreatedBy,
	}
	if task.SourceImageID != nil && s.isCleanSourceFile(ctx, opts.TenantID, *task.SourceImageID) {
		opts.SourceFileID = *task.SourceImageID
	}
	var fr *files.FileRecord
	var saveErr error
	if opts.SourceFileID != uuid.Nil {
		fr, saveErr = s.Files.SaveProcessed(ctx, opts)
	} else {
		fr, saveErr = s.Files.SaveUntrustedProcessed(ctx, opts)
	}
	if saveErr != nil {
		return "", nil, "", saveErr
	}
	idCopy := fr.ID
	return strings.TrimSpace(fr.PublicURL), &idCopy, strings.TrimSpace(fr.ObjectKey), nil
}

func (s *Service) persistTenantID(ctx context.Context, task *ImageTask) int64 {
	if s == nil || s.DB == nil || task == nil {
		return 0
	}
	var resourceTenant int64
	if task.ProductID != nil && *task.ProductID != uuid.Nil {
		var p product.Product
		if err := s.DB.WithContext(ctx).Select("tenant_id").First(&p, "id = ?", *task.ProductID).Error; err == nil {
			resourceTenant = p.TenantID
		}
	}
	if resourceTenant <= 0 && task.SourceImageID != nil && *task.SourceImageID != uuid.Nil {
		var source files.FileRecord
		if err := s.DB.WithContext(ctx).Select("tenant_id").First(&source, "id = ?", *task.SourceImageID).Error; err == nil {
			resourceTenant = source.TenantID
		}
	}
	var actorTenant int64
	if task.CreatedBy != nil && *task.CreatedBy != uuid.Nil {
		var actor admin.AdminUser
		if err := s.DB.WithContext(ctx).Select("tenant_id").First(&actor, "id = ?", *task.CreatedBy).Error; err == nil {
			actorTenant = actor.TenantID
		}
	}
	if task.TenantID > 0 {
		if resourceTenant > 0 && resourceTenant != task.TenantID {
			return 0
		}
		if actorTenant > 0 && actorTenant != task.TenantID {
			return 0
		}
		return task.TenantID
	}
	if resourceTenant > 0 && actorTenant > 0 && resourceTenant != actorTenant {
		return 0
	}
	if resourceTenant > 0 {
		return resourceTenant
	}
	return actorTenant
}

func (s *Service) isCleanSourceFile(ctx context.Context, tenantID int64, id uuid.UUID) bool {
	if s == nil || s.DB == nil || tenantID <= 0 || id == uuid.Nil {
		return false
	}
	var n int64
	return s.DB.WithContext(ctx).Model(&files.FileRecord{}).Where("id = ? AND tenant_id = ? AND security_status = ?", id, tenantID, files.SecurityClean).Count(&n).Error == nil && n == 1
}

func downloadResultBytes(ctx context.Context, rawURL string) ([]byte, string, error) {
	u := strings.TrimSpace(rawURL)
	if u == "" {
		return nil, "", fmt.Errorf("empty url")
	}
	opts := safedownload.DefaultOptions()
	opts.MaxBodyBytes = 30 << 20
	opts.ResponseTimeout = 90 * time.Second
	result, err := safedownload.Download(ctx, u, opts)
	if err != nil {
		return nil, "", fmt.Errorf("download result: %w", err)
	}
	return result.Data, result.ContentType, nil
}

func (s *Service) upsertPrimaryTaskItem(ctx context.Context, task *ImageTask, finalURL, storageKey string, fileID *uuid.UUID, scoreJSON []byte, selectedBest bool) error {
	if s == nil || s.DB == nil || task == nil {
		return nil
	}
	var existing ImageTaskItem
	err := s.DB.WithContext(ctx).Where("task_id = ?", task.ID).Order("created_at ASC").First(&existing).Error
	if err != nil {
		item := &ImageTaskItem{
			TenantID:         task.TenantID,
			TaskID:           task.ID,
			ProductID:        task.ProductID,
			SourceImageID:    task.SourceImageID,
			SourceImageURL:   task.SourceImageURL,
			OutputImageURL:   finalURL,
			OutputStorageKey: storageKey,
			OutputFileID:     fileID,
			Status:           ItemStatusSuccess,
			IsSelectedBest:   selectedBest,
		}
		if len(scoreJSON) > 0 {
			item.ScoreJSON = scoreJSON
		}
		return s.DB.WithContext(ctx).Create(item).Error
	}
	updates := map[string]any{
		"output_image_url":   finalURL,
		"output_storage_key": storageKey,
		"output_file_id":     fileID,
		"status":             ItemStatusSuccess,
		"error_message":      "",
		"is_selected_best":   selectedBest,
	}
	if len(scoreJSON) > 0 {
		updates["score_json"] = scoreJSON
	}
	return s.DB.WithContext(ctx).Model(&ImageTaskItem{}).Where("id = ?", existing.ID).Updates(updates).Error
}

func (s *Service) finalizeTaskSuccess(ctx context.Context, _ interface{}, task *ImageTask, finalURL string, fileID *uuid.UUID, storageKey string, outObj map[string]any, scoreJSON []byte, selectedBest bool) error {
	return s.finalizeTaskSuccessWithStatus(ctx, task, finalURL, fileID, storageKey, outObj, scoreJSON, selectedBest, StatusSuccess)
}

func (s *Service) finalizeTaskSuccessWithStatus(ctx context.Context, task *ImageTask, finalURL string, fileID *uuid.UUID, storageKey string, outObj map[string]any, scoreJSON []byte, selectedBest bool, status string) error {
	if outObj == nil {
		outObj = map[string]any{}
	}
	outObj["resultUrl"] = finalURL
	outObj["storageKey"] = storageKey
	if fileID != nil {
		outObj["resultFileId"] = fileID.String()
	}
	if len(scoreJSON) > 0 {
		outObj["score"] = json.RawMessage(scoreJSON)
	}
	if strings.TrimSpace(status) == "" {
		status = StatusSuccess
	}
	outBytes, _ := json.Marshal(outObj)
	fin := time.Now().UTC()
	updates := map[string]any{
		"status":            status,
		"output":            outBytes,
		"result_url":        finalURL,
		"error_message":     "",
		"finished_at":       &fin,
		"completed_at":      &fin,
		"result_count":      1,
		"retry_count":       0,
		"next_retry_at":     nil,
		"retry_enqueued_at": nil,
		"locked_by":         nil,
		"locked_until":      nil,
	}
	if fileID != nil {
		updates["result_file_id"] = fileID
	}
	workerID, claim := imageLeaseFrom(ctx)
	if claim != nil && strings.TrimSpace(workerID) != "" {
		if err := s.finishImageTask(ctx, task.ID, workerID, claim, updates); err != nil {
			return err
		}
	} else if err := s.DB.WithContext(ctx).Model(&ImageTask{}).Where("id = ?", task.ID).Updates(updates).Error; err != nil {
		return err
	}
	_ = s.upsertPrimaryTaskItem(ctx, task, finalURL, storageKey, fileID, scoreJSON, selectedBest)
	hints := inputHints(task.Input)
	s.maybeAutoApply(ctx, task, hints)
	return nil
}

func extractPromptFields(p CreatePayload) (prompt, neg, inputMode string) {
	h := inputHints(p.Input)
	prompt = stringFromMap(h, "prompt")
	neg = stringFromMap(h, "negativePrompt")
	inputMode = stringFromMap(h, "inputMode")
	if prompt == "" {
		prompt = stringFromMap(h, "assembled_prompt")
	}
	return prompt, neg, inputMode
}

func (s *Service) createTaskItemPending(ctx context.Context, taskID uuid.UUID, productID *uuid.UUID, srcID *uuid.UUID, srcURL string) (*uuid.UUID, error) {
	var task ImageTask
	if err := s.DB.WithContext(ctx).Select("tenant_id").First(&task, "id = ?", taskID).Error; err != nil {
		return nil, err
	}
	item := &ImageTaskItem{
		TenantID:       task.TenantID,
		TaskID:         taskID,
		ProductID:      productID,
		SourceImageID:  srcID,
		SourceImageURL: srcURL,
		Status:         ItemStatusPending,
	}
	if err := s.DB.WithContext(ctx).Create(item).Error; err != nil {
		return nil, err
	}
	id := item.ID
	return &id, nil
}

func (s *Service) markTaskItemFailed(ctx context.Context, taskID uuid.UUID, msg string) {
	_ = s.DB.WithContext(ctx).Model(&ImageTaskItem{}).
		Where("task_id = ? AND status IN ?", taskID, []string{ItemStatusPending, ItemStatusRunning}).
		Updates(map[string]any{"status": ItemStatusFailed, "error_message": truncateRunes(msg, 4000)}).Error
}

// resolveOpenAIEditSource resolves bytes for openai edit operations.
func (s *Service) resolveOpenAIEditSource(ctx context.Context, task *ImageTask) (removeBGSource, error) {
	return s.resolveOpenAIReplaceBackgroundSource(ctx, task)
}

func ptrUUID(u uuid.UUID) *uuid.UUID {
	if u == uuid.Nil {
		return nil
	}
	return &u
}
