package api

import (
	"net/http"

	"github.com/QiuShichang/auto-finance-assistant/internal/backup"
	"github.com/QiuShichang/auto-finance-assistant/internal/chat"
	"github.com/QiuShichang/auto-finance-assistant/internal/knowledge"
	"github.com/QiuShichang/auto-finance-assistant/internal/modelclient"
	"github.com/QiuShichang/auto-finance-assistant/internal/queue"
	"github.com/QiuShichang/auto-finance-assistant/internal/rag"
	"github.com/QiuShichang/auto-finance-assistant/internal/web"
)

// version 由 main 通过 ldflags 注入。
var version = "dev"

// Server 持有所有处理器依赖（仿 go-rmm 的 handler-as-method 风格）。
type Server struct {
	chat          *chat.Service
	model         modelclient.ModelClient
	queue         *queue.LLMQueue
	importer      *knowledge.Importer
	vector        *rag.VectorSearcher
	backup        *backup.Manager
	adminPassword string
}

// New 构造 Server。
func New(c *chat.Service, o modelclient.ModelClient, q *queue.LLMQueue, imp *knowledge.Importer) *Server {
	return &Server{chat: c, model: o, queue: q, importer: imp}
}

// SetVectorSearcher 注入向量检索器（M6）。
func (s *Server) SetVectorSearcher(v *rag.VectorSearcher) { s.vector = v }

// Register 把路由注册到 mux。使用 Go 1.22 方法路由。
func (s *Server) Register(mux *http.ServeMux) {
	// 系统
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/system/info", s.AuthMiddleware(s.handleSystemInfo))
	mux.HandleFunc("GET /api/system/model", s.handleSystemModel)

	// 会话
	mux.HandleFunc("POST /api/conversations", s.handleCreateConversation)
	mux.HandleFunc("GET /api/conversations", s.handleListConversations)
	mux.HandleFunc("GET /api/conversations/{id}", s.handleGetConversation)

	// 聊天
	mux.HandleFunc("POST /api/chat", s.handleChat)
	mux.HandleFunc("POST /api/chat/stream", s.handleChatStream)

	// FAQ（读开放，写需认证）
	mux.HandleFunc("POST /api/faqs", s.AuthMiddleware(s.handleCreateFAQ))
	mux.HandleFunc("GET /api/faqs", s.handleListFAQs)
	mux.HandleFunc("GET /api/faqs/{id}", s.handleGetFAQ)
	mux.HandleFunc("PUT /api/faqs/{id}", s.AuthMiddleware(s.handleUpdateFAQ))
	mux.HandleFunc("DELETE /api/faqs/{id}", s.AuthMiddleware(s.handleDeleteFAQ))
	mux.HandleFunc("POST /api/faqs/import", s.AuthMiddleware(s.handleImportFAQs))
	mux.HandleFunc("POST /api/faqs/test", s.handleTestFAQMatch)

	// 文档（读开放，写需认证）
	mux.HandleFunc("POST /api/documents", s.AuthMiddleware(s.handleUploadDocument))
	mux.HandleFunc("GET /api/documents", s.handleListDocuments)
	mux.HandleFunc("GET /api/documents/{id}", s.handleGetDocument)
	mux.HandleFunc("PUT /api/documents/{id}", s.AuthMiddleware(s.handleUpdateDocument))
	mux.HandleFunc("DELETE /api/documents/{id}", s.AuthMiddleware(s.handleDeleteDocument))
	mux.HandleFunc("POST /api/documents/{id}/publish", s.AuthMiddleware(s.handlePublishDocument))
	mux.HandleFunc("POST /api/documents/{id}/disable", s.AuthMiddleware(s.handleDisableDocument))
	mux.HandleFunc("POST /api/documents/{id}/reparse", s.AuthMiddleware(s.handleReparseDocument))
	mux.HandleFunc("GET /api/documents/{id}/chunks", s.handleListChunks)

	// 向量化（M6，需认证）
	mux.HandleFunc("POST /api/documents/{id}/embed", s.AuthMiddleware(s.handleEmbedDocument))

	// 金融计算（M5，开放）
	mux.HandleFunc("POST /api/finance/equal-payment", s.handleEqualPayment)
	mux.HandleFunc("POST /api/finance/equal-principal", s.handleEqualPrincipal)
	mux.HandleFunc("POST /api/finance/down-payment", s.handleDownPayment)

	// 反馈（M7，提交开放，查看需认证）
	mux.HandleFunc("POST /api/feedback", s.handleCreateFeedback)
	mux.HandleFunc("GET /api/feedback", s.AuthMiddleware(s.handleListFeedback))

	// 审计与管理（M7，需认证）
	mux.HandleFunc("GET /api/audit/logs", s.AuthMiddleware(s.handleListAuditLogs))
	mux.HandleFunc("GET /api/refused", s.AuthMiddleware(s.handleListRefused))
	mux.HandleFunc("GET /api/metrics", s.AuthMiddleware(s.handleMetrics))

	// 备份（M9，需认证）
	mux.HandleFunc("POST /api/system/backup", s.AuthMiddleware(s.handleBackup))
	mux.HandleFunc("GET /api/system/backups", s.AuthMiddleware(s.handleListBackups))

	// 前端静态资源（SPA）
	mux.Handle("/", web.StaticHandler())
}

// SetVersion 供 main 注入版本号。
func SetVersion(v string) { version = v }
