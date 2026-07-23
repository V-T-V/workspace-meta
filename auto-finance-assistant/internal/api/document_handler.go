package api

import (
	"errors"
	"net/http"

	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

// documentResponse 文档对外结构。
type documentResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	OriginalName  string `json:"originalName"`
	FileType      string `json:"fileType"`
	FileSize      int64  `json:"fileSize"`
	Version       string `json:"version"`
	Institution   string `json:"institution"`
	ProductCode   string `json:"productCode"`
	Status        string `json:"status"`
	ChunkCount    int    `json:"chunkCount"`
	EffectiveDate string `json:"effectiveDate"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

func toDocumentResponse(d *storage.Document, chunkCount int) documentResponse {
	return documentResponse{
		ID: d.ID, Name: d.Name, OriginalName: d.OriginalName, FileType: d.FileType,
		FileSize: d.FileSize, Version: d.Version, Institution: d.Institution,
		ProductCode: d.ProductCode, Status: d.Status, ChunkCount: chunkCount,
		EffectiveDate: d.EffectiveDate,
		CreatedAt: d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: d.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (s *Server) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	// 限制大小
	maxBytes := int64(50) * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	if err := r.ParseMultipartForm(maxBytes); err != nil {
		writeError(w, http.StatusBadRequest, "upload_failed", "文件过大或格式错误: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "no_file", "未提供 file 字段")
		return
	}
	defer file.Close()

	result, err := s.importer.Import(r.Context(), file, header.Filename)
	if err != nil {
		writeError(w, http.StatusBadRequest, "import_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	docs, err := storage.ListDocuments(r.Context(), s.importer.DB(), status, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "查询文档列表失败")
		return
	}
	out := make([]documentResponse, 0, len(docs))
	for _, d := range docs {
		n, _ := storage.CountChunksByDocument(r.Context(), s.importer.DB(), d.ID)
		out = append(out, toDocumentResponse(d, n))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "total": len(out)})
}

func (s *Server) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, err := storage.GetDocument(r.Context(), s.importer.DB(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "文档不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", "查询文档失败")
		return
	}
	n, _ := storage.CountChunksByDocument(r.Context(), s.importer.DB(), id)
	writeJSON(w, http.StatusOK, toDocumentResponse(d, n))
}

type updateDocumentRequest struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	Institution   string `json:"institution"`
	ProductCode   string `json:"productCode"`
	Region        string `json:"region"`
	CustomerType  string `json:"customerType"`
	EffectiveDate string `json:"effectiveDate"`
	ExpiryDate    string `json:"expiryDate"`
}

func (s *Server) handleUpdateDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body updateDocumentRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体格式错误")
		return
	}
	d, err := storage.GetDocument(r.Context(), s.importer.DB(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "文档不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", "查询失败")
		return
	}
	if body.Name != "" {
		d.Name = body.Name
	}
	d.Version = body.Version
	d.Institution = body.Institution
	d.ProductCode = body.ProductCode
	d.Region = body.Region
	d.CustomerType = body.CustomerType
	d.EffectiveDate = body.EffectiveDate
	d.ExpiryDate = body.ExpiryDate
	if err := storage.UpdateDocumentMeta(r.Context(), s.importer.DB(), d); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	n, _ := storage.CountChunksByDocument(r.Context(), s.importer.DB(), id)
	writeJSON(w, http.StatusOK, toDocumentResponse(d, n))
}

func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.importer.Delete(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "文档不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}

func (s *Server) handlePublishDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.importer.Publish(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, "publish_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"published": true, "id": id})
}

func (s *Server) handleDisableDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.importer.Disable(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, "disable_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"disabled": true, "id": id})
}

func (s *Server) handleReparseDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.importer.ReParse(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, "reparse_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reparsed": true, "id": id})
}

type chunkResponse struct {
	ID         string `json:"id"`
	Sequence   int    `json:"sequence"`
	Title      string `json:"title"`
	Section    string `json:"section"`
	Content    string `json:"content"`
	PageNumber int    `json:"pageNumber"`
	TokenCount int    `json:"tokenCount"`
}

func (s *Server) handleListChunks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	chunks, err := storage.ListChunksByDocument(r.Context(), s.importer.DB(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "查询片段失败")
		return
	}
	out := make([]chunkResponse, 0, len(chunks))
	for _, c := range chunks {
		out = append(out, chunkResponse{
			ID: c.ID, Sequence: c.Sequence, Title: c.Title, Section: c.Section,
			Content: c.Content, PageNumber: c.PageNumber, TokenCount: c.TokenCount,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "total": len(out)})
}
