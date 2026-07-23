-- 003_fts.sql · M4 全文检索（FTS5）
-- 对应原计划 7.3。M1 阶段存在但不激活。
-- 用 trigram tokenizer：对中文友好（按 3 字符切分），优于 unicode61（中文不分词）。

CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
    chunk_id UNINDEXED,
    title,
    section,
    content,
    tokenize = 'trigram'
);

-- 同步触发器：chunks 增删改时自动维护 chunks_fts。
-- 索引全部片段，查询时再按文档状态（active）过滤。

CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
    INSERT INTO chunks_fts(chunk_id, title, section, content)
    VALUES (new.id, COALESCE(new.title,''), COALESCE(new.section,''), new.content);
END;

CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
    DELETE FROM chunks_fts WHERE chunk_id = old.id;
END;

CREATE TRIGGER IF NOT EXISTS chunks_au AFTER UPDATE ON chunks BEGIN
    DELETE FROM chunks_fts WHERE chunk_id = old.id;
    INSERT INTO chunks_fts(chunk_id, title, section, content)
    VALUES (new.id, COALESCE(new.title,''), COALESCE(new.section,''), new.content);
END;

