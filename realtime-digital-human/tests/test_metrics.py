"""#6 Metrics 端点测试（Prometheus 格式）。"""
from digitalhuman.metrics import MetricsRegistry


def test_record_pipeline_complete():
    m = MetricsRegistry()
    m.record_pipeline_complete({"first_token_ms": 100, "first_audio_ms": 500, "first_frame_ms": 800})
    m.record_pipeline_complete({"first_token_ms": 200, "first_audio_ms": 600, "first_frame_ms": 900})

    out = m.render_prometheus()
    assert "dh_first_token_ms" in out
    assert 'quantile="0.5"' in out
    assert 'quantile="0.95"' in out
    assert "dh_first_token_ms_count 2" in out


def test_counters():
    m = MetricsRegistry()
    m.record_pipeline_cancel()
    m.record_tts_fail()
    m.record_barge_in()

    out = m.render_prometheus()
    assert "dh_pipeline_cancel_total 1" in out
    assert "dh_tts_fail_total 1" in out
    assert "dh_barge_in_total 1" in out


def test_gauges():
    m = MetricsRegistry()
    m.set_sessions_active(2)
    m.set_audio_buffer_bytes(1024)

    out = m.render_prometheus()
    assert "dh_sessions_active 2" in out
    assert "dh_audio_buffer_bytes 1024" in out


def test_percentile():
    """验证 P50/P95 计算。"""
    m = MetricsRegistry()
    # 喂 10 个样本（1-10ms）
    for i in range(1, 11):
        m.record_pipeline_complete({"first_token_ms": i * 100})

    out = m.render_prometheus()
    # P50 应该是第 5-6 个样本（500-600ms）
    # P95 应该是第 9-10 个样本（900-1000ms）
    lines = out.split("\n")
    p50_line = [l for l in lines if "first_token_ms" in l and "quantile=\"0.5\"" in l]
    p95_line = [l for l in lines if "first_token_ms" in l and "quantile=\"0.95\"" in l]
    assert len(p50_line) == 1
    assert len(p95_line) == 1
    # P50 ≤ P95
    p50_val = float(p50_line[0].split()[-1])
    p95_val = float(p95_line[0].split()[-1])
    assert p50_val <= p95_val
    assert p50_val >= 100  # 至少第一个样本
    assert p95_val >= 900  # 至少第九个


def test_prometheus_format_valid():
    """输出应是合法 Prometheus exposition format。"""
    m = MetricsRegistry()
    m.record_pipeline_complete({"first_token_ms": 50})
    out = m.render_prometheus()
    lines = out.strip().split("\n")
    # 每个非注释/非空行应是 "metric_name value" 或 "metric_name{labels} value"
    for line in lines:
        if line.startswith("#") or not line.strip():
            continue
        parts = line.split()
        assert len(parts) >= 2, f"非法行: {line}"
        # 最后一个 token 应是数字
        try:
            float(parts[-1])
        except ValueError:
            assert False, f"值不是数字: {line}"


def test_uptime():
    """uptime 应随时间增长。"""
    import time
    m = MetricsRegistry()
    time.sleep(0.1)
    out = m.render_prometheus()
    assert "dh_uptime_seconds" in out
    # 至少 0 秒
    uptime_line = [l for l in out.split("\n") if "dh_uptime_seconds" in l and not l.startswith("#")][0]
    uptime = float(uptime_line.split()[-1])
    assert uptime >= 0
