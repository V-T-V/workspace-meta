package api

import (
	"errors"
	"net/http"

	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

// handleEmbedDocument 对文档片段批量生成向量（M6）。
func (s *Server) handleEmbedDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if s.vector == nil {
		writeError(w, http.StatusServiceUnavailable, "vector_disabled", "向量检索未启用")
		return
	}
	doc, err := storage.GetDocument(r.Context(), s.importer.DB(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "文档不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", "查询文档失败")
		return
	}
	if doc.Status != storage.DocStatusActive {
		writeError(w, http.StatusBadRequest, "not_active", "文档未发布，无法向量化")
		return
	}

	count, err := s.vector.EmbedAndStore(r.Context(), id, 8)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "embed_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"documentId":  id,
		"embedded":    count,
		"vectorCount": s.vector.Index().Size(),
	})
}
