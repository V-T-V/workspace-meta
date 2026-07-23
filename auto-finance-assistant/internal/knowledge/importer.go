package knowledge

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/QiuShichang/auto-finance-assistant/internal/config"
	"github.com/QiuShichang/auto-finance-assistant/internal/parser"
	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

// Importer 编排文档导入：保存文件 → hash 去重 → 解析 → 分块 → 落库。
// 对应原计划 3.2 文档导入链路。
type Importer struct {
	db      *sql.DB
	parser  *parser.Registry
	chunker *Chunker
	cfg     config.DocumentsConfig
	docDir  string
	tmpDir  string
	log     *slog.Logger
}

// NewImporter 构造。
func NewImporter(db *sql.DB, reg *parser.Registry, cfg config.DocumentsConfig, docDir, tmpDir string, log *slog.Logger) *Importer {
	return &Importer{
		db:      db,
		parser:  reg,
		chunker: NewChunker(cfg.ChunkMinChars, cfg.ChunkMaxChars, cfg.ChunkOverlapChars),
		cfg:     cfg,
		docDir:  docDir,
		tmpDir:  tmpDir,
		log:     log,
	}
}

// DB 返回底层 db 句柄（供 handler 直接查询文档/片段用）。
func (im *Importer) DB() *sql.DB { return im.db }

// ImportResult 是一次导入的结果。
type ImportResult struct {
	DocumentID string
	Status     string // active(已解析) | draft(仅保存未解析) | duplicate(重复)
	ChunkCount int
	Message    string
}

// Import 从 reader 读取上传文件并导入。
// originalName 是用户上传的文件名；hash 去重。
func (im *Importer) Import(ctx context.Context, reader io.Reader, originalName string) (*ImportResult, error) {
	ext := strings.ToLower(filepath.Ext(originalName))
	if ext == "" {
		return nil, fmt.Errorf("[knowledge] 文件无扩展名")
	}
	if !im.isAllowed(ext) {
		return nil, fmt.Errorf("[knowledge] 不支持的扩展名 %s", ext)
	}

	// 1. 写入临时文件
	tmpPath := filepath.Join(im.tmpDir, randomName()+ext)
	f, err := os.Create(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("[knowledge] 创建临时文件失败: %w", err)
	}
	size, err := io.Copy(f, reader)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("[knowledge] 写入临时文件失败: %w", err)
	}
	_ = f.Close()

	// 大小校验
	if maxBytes := int64(im.cfg.MaxFileSizeMB) * 1024 * 1024; size > maxBytes {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("[knowledge] 文件超过 %dMB 限制", im.cfg.MaxFileSizeMB)
	}

	// 2. 计算 hash
	hash, err := fileHash(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}

	// 3. 去重检查
	if existing, err := storage.GetDocumentByHash(ctx, im.db, hash); err == nil && existing != nil {
		_ = os.Remove(tmpPath)
		return &ImportResult{
			DocumentID: existing.ID,
			Status:     "duplicate",
			Message:    fmt.Sprintf("文件已存在（文档 %s）", existing.ID),
		}, nil
	} else if err != nil && !errors.Is(err, storage.ErrNotFound) {
		_ = os.Remove(tmpPath)
		return nil, err
	}

	// 4. 移动到 documents 目录
	docID := randomName()
	finalPath := filepath.Join(im.docDir, docID+ext)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, fmt.Errorf("[knowledge] 移动文件失败: %w", err)
	}

	// 5. 落库文档记录（draft）
	name := strings.TrimSuffix(originalName, filepath.Ext(originalName))
	doc := &storage.Document{
		ID:           docID,
		Name:         name,
		OriginalName: originalName,
		FilePath:     finalPath,
		FileType:     ext,
		FileSize:     size,
		FileHash:     hash,
		Status:       storage.DocStatusProcessing,
		Metadata:     "{}",
	}
	if err := storage.CreateDocument(ctx, im.db, doc); err != nil {
		_ = os.Remove(finalPath)
		return nil, err
	}

	// 6. 解析 + 分块
	parsed, err := im.parser.Parse(ctx, finalPath)
	if err != nil {
		if e := storage.UpdateDocumentStatus(ctx, im.db, docID, storage.DocStatusFailed); e != nil {
			im.log.Error("[knowledge] 标记文档失败状态也失败", "docId", docID, "err", e)
		}
		return &ImportResult{
			DocumentID: docID, Status: "failed",
			Message: fmt.Sprintf("解析失败：%s", err.Error()),
		}, nil
	}

	chunks := im.chunker.Chunk(docID, parsed)
	if len(chunks) > 0 {
		if err := storage.CreateChunks(ctx, im.db, chunks); err != nil {
			im.log.Error("[knowledge] 分块落库失败", "docId", docID, "err", err)
			if e := storage.UpdateDocumentStatus(ctx, im.db, docID, storage.DocStatusFailed); e != nil {
				im.log.Error("[knowledge] 标记文档失败状态也失败", "docId", docID, "err", e)
			}
			return &ImportResult{
				DocumentID: docID, Status: "failed",
				Message: "分块落库失败：" + err.Error(),
			}, nil
		}
	}

	// 解析完成，状态保持 draft（需人工发布才进入检索）
	if e := storage.UpdateDocumentStatus(ctx, im.db, docID, storage.DocStatusDraft); e != nil {
		im.log.Error("[knowledge] 标记文档 draft 状态失败", "docId", docID, "err", e)
	}
	im.log.Info("[knowledge] 文档导入完成", "docId", docID, "name", originalName,
		"blocks", len(parsed.Blocks), "chunks", len(chunks))

	return &ImportResult{
		DocumentID: docID,
		Status:     "draft",
		ChunkCount: len(chunks),
		Message:    fmt.Sprintf("已解析 %d 个片段，待发布", len(chunks)),
	}, nil
}

// Publish 发布文档：draft → active。
// 对应原计划"文档发布后进入检索"。
func (im *Importer) Publish(ctx context.Context, docID string) error {
	doc, err := storage.GetDocument(ctx, im.db, docID)
	if err != nil {
		return err
	}
	if doc.Status != storage.DocStatusDraft && doc.Status != storage.DocStatusInactive {
		return fmt.Errorf("[knowledge] 文档状态 %s 不可发布（需 draft 或 inactive）", doc.Status)
	}
	return storage.UpdateDocumentStatus(ctx, im.db, docID, storage.DocStatusActive)
}

// Disable 停用文档：active → inactive。
func (im *Importer) Disable(ctx context.Context, docID string) error {
	return storage.UpdateDocumentStatus(ctx, im.db, docID, storage.DocStatusInactive)
}

// ReParse 重新解析文档（删除旧片段，重新分块）。
func (im *Importer) ReParse(ctx context.Context, docID string) error {
	doc, err := storage.GetDocument(ctx, im.db, docID)
	if err != nil {
		return err
	}
	_ = storage.DeleteChunksByDocument(ctx, im.db, docID)
	_ = storage.UpdateDocumentStatus(ctx, im.db, docID, storage.DocStatusProcessing)

	parsed, err := im.parser.Parse(ctx, doc.FilePath)
	if err != nil {
		_ = storage.UpdateDocumentStatus(ctx, im.db, docID, storage.DocStatusFailed)
		return err
	}
	chunks := im.chunker.Chunk(docID, parsed)
	if err := storage.CreateChunks(ctx, im.db, chunks); err != nil {
		return err
	}
	_ = storage.UpdateDocumentStatus(ctx, im.db, docID, storage.DocStatusDraft)
	return nil
}

// Delete 删除文档及其片段与文件。
func (im *Importer) Delete(ctx context.Context, docID string) error {
	doc, err := storage.GetDocument(ctx, im.db, docID)
	if err != nil {
		return err
	}
	if err := storage.DeleteDocument(ctx, im.db, docID); err != nil {
		return err
	}
	_ = os.Remove(doc.FilePath)
	return nil
}

func (im *Importer) isAllowed(ext string) bool {
	for _, allowed := range im.cfg.AllowedExtensions {
		if strings.EqualFold(ext, allowed) {
			return true
		}
	}
	return false
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func randomName() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")[:24]
}
