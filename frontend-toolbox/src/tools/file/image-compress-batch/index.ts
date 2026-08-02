import type { Tool, ToolInstance, ToolContext } from '../../../types.ts';
import meta from './meta.ts';
import { createToolLayout } from '../../../ui/layout.ts';
import { input, button, checkbox, select, el, downloadBlob } from '../../../ui/components.ts';
import { createFileDrop, formatBytes, type FileWithPath } from '../../../ui/file-drop.ts';
import { compressImage, type ImageFormat } from '../../../lib/image-utils.ts';
import { toastSuccess, toastError } from '../../../ui/toast.ts';
import JSZip from 'jszip';

interface FileItem {
  wp: FileWithPath;
  status: 'pending' | 'processing' | 'done' | 'error';
  result?: { blob: Blob; width: number; height: number; format: ImageFormat };
  error?: string;
}

const tool: Tool = {
  meta,
  create(): ToolInstance {
    return {
      mount(ctx: ToolContext) {
        const layout = createToolLayout(meta);

        // ---- 设置 ----
        const qualityInp = input('', '0.7');
        qualityInp.type = 'number'; qualityInp.step = '0.05'; qualityInp.min = '0.1'; qualityInp.max = '1';
        qualityInp.style.width = '80px';

        const pngModeSel = select([
          ['png8', '256色（截图神器）'],
          ['png-best', '无损优化'],
          ['png', '不优化'],
        ], 'png8');
        const pngModeLabel = el('label', undefined, 'PNG模式');
        pngModeLabel.style.cssText = 'font-size:13px;margin:0 4px 0 12px';

        const { wrapper: sizeWrap, input: sizeCb } = checkbox('限制最大宽度', false);
        const maxWidthInp = input('', '1280');
        maxWidthInp.type = 'number'; maxWidthInp.min = '1'; maxWidthInp.style.width = '80px';
        maxWidthInp.style.display = 'none';
        sizeCb.addEventListener('change', () => { maxWidthInp.style.display = sizeCb.checked ? '' : 'none'; });

        // ---- 文件列表 ----
        const listWrap = el('div');
        listWrap.style.cssText = 'margin-top:12px;max-height:360px;overflow-y:auto;border:1px solid var(--border);border-radius:8px;display:none';
        const listTable = el('table');
        listTable.style.cssText = 'width:100%;border-collapse:collapse;font-size:13px';
        listWrap.append(listTable);

        // ---- 压缩按钮 ----
        const btnBar = el('div', 'ftb-toolbar');
        btnBar.style.marginTop = '8px';
        const compressBtn = button('🚀 开始压缩', () => doCompress(), 'primary');
        compressBtn.style.display = 'none';
        const clearBtn = button('🗑️ 清空', () => clearAll(), 'ghost');
        clearBtn.style.display = 'none';
        const progressText = el('span');
        progressText.style.cssText = 'font-size:13px;color:var(--text-soft)';
        btnBar.append(progressText, compressBtn, clearBtn);

        // ---- 结果区 ----
        const resultGrid = el('div', 'ftb-batch-grid');
        const actions = el('div', 'ftb-toolbar');
        actions.style.marginTop = '12px';

        let items: FileItem[] = [];

        // ---- 清空列表 ----
        function clearAll(): void {
          items = [];
          listTable.replaceChildren();
          listWrap.style.display = 'none';
          resultGrid.replaceChildren();
          actions.replaceChildren();
          compressBtn.style.display = 'none';
          clearBtn.style.display = 'none';
          progressText.textContent = '';
        }

        // ---- 拖拽区 ----
        const drop = createFileDrop({
          accept: 'image/*',
          multiple: true,
          directory: true,
          hint: '点击选择文件夹 或 拖入多张图片',
          onFilesWithPath(files: FileWithPath[]) {
            const filtered = files.filter((f) => f.file.type.startsWith('image/'));
            if (!filtered.length) { toastError('未找到图片文件'); return; }
            items = filtered.map((wp) => ({ wp, status: 'pending' as const }));
            resultGrid.replaceChildren(el('div', 'ftb-desc', '（调整参数后点击压缩）'));
            actions.replaceChildren();
            renderList();
            compressBtn.style.display = '';
            compressBtn.textContent = `🚀 开始压缩（${items.length} 张）`;
            clearBtn.style.display = '';
          },
        });

        // ---- 渲染文件列表 ----
        function renderList(): void {
          listTable.replaceChildren();
          if (!items.length) { listWrap.style.display = 'none'; return; }
          listWrap.style.display = '';
          const thead = el('thead');
          const hr = el('tr');
          for (const h of ['文件', '大小', '状态']) hr.append(el('th', undefined, h));
          thead.append(hr);
          listTable.append(thead);
          const tbody = el('tbody');
          for (const item of items) {
            const tr = el('tr');
            tr.style.cssText = 'border-bottom:1px solid var(--border)';
            const tdName = el('td');
            tdName.style.cssText = 'padding:6px 10px;max-width:280px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap';
            tdName.textContent = item.wp.relativePath || item.wp.file.name;
            tdName.title = item.wp.relativePath || item.wp.file.name;
            const tdSize = el('td');
            tdSize.style.cssText = 'padding:6px 10px;white-space:nowrap;color:var(--text-muted)';
            tdSize.textContent = formatBytes(item.wp.file.size);
            const tdStatus = el('td');
            tdStatus.style.cssText = 'padding:6px 10px;white-space:nowrap;min-width:100px';
            tdStatus.textContent = '⏳ 等待';
            tr.append(tdName, tdSize, tdStatus);
            tbody.append(tr);
          }
          listTable.append(tbody);
        }

        function updateRow(idx: number): void {
          const tbody = listTable.querySelector('tbody');
          if (!tbody) return;
          const row = tbody.children[idx] as HTMLTableRowElement | undefined;
          if (!row) return;
          const item = items[idx]!;
          const tdStatus = row.children[2] as HTMLTableCellElement | undefined;
          if (!tdStatus) return;
          if (item.status === 'processing') tdStatus.textContent = '🔄 压缩中…';
          else if (item.status === 'error') tdStatus.textContent = '❌ ' + (item.error ?? '');
          else if (item.result) {
            const saved = item.result.blob.size < item.wp.file.size;
            const pct = Math.abs((1 - item.result.blob.size / item.wp.file.size) * 100).toFixed(0);
            tdStatus.textContent = `✅ ${formatBytes(item.result.blob.size)} ${saved ? '↓' + pct + '%' : '↑' + pct + '%'}`;
          }
        }

        // ---- 逐张压缩 ----
        async function doCompress(): Promise<void> {
          if (!items.length) return;
          const q = Number(qualityInp.value) || 0.7;
          const maxW = sizeCb.checked ? Number(maxWidthInp.value) || undefined : undefined;
          const pngMode = pngModeSel.value;

          const pickFormat = (file: File): ImageFormat => {
            if (file.type === 'image/png') {
              if (pngMode === 'png-best') return 'image/png-best';
              if (pngMode === 'png8') return 'image/png8';
              return 'image/png';
            }
            if (file.type === 'image/webp') return 'image/webp';
            return 'image/jpeg';
          };

          // 重新压缩前重置所有项状态
          for (const it of items) {
            it.status = 'pending';
            it.result = undefined;
            it.error = undefined;
          }
          renderList();

          compressBtn.style.display = 'none';
          resultGrid.replaceChildren();
          actions.replaceChildren();

          let done = 0;
          const totalItems = items.length;
          progressText.textContent = `0 / ${totalItems}`;

          for (let i = 0; i < items.length; i++) {
            const item = items[i]!;
            item.status = 'processing';
            updateRow(i);
            progressText.textContent = `${done} / ${totalItems}`;
            // yield to let browser render the update
            await new Promise((r) => setTimeout(r, 30));

            try {
              const { blob, width, height } = await compressImage(item.wp.file, {
                quality: q, maxWidth: maxW, format: pickFormat(item.wp.file),
              });
              item.result = { blob, width, height, format: pickFormat(item.wp.file) };
              item.status = 'done';
            } catch (e) {
              item.status = 'error';
              item.error = (e as Error).message;
            }
            done++;
            updateRow(i);
            progressText.textContent = `${done} / ${totalItems}`;
          }

          compressBtn.style.display = '';
          compressBtn.textContent = '🔄 重新压缩';
          progressText.textContent = '';
          renderResults();
        }

        // ---- 结果网格 ----
        function renderResults(): void {
          resultGrid.replaceChildren();
          actions.replaceChildren();
          const doneItems = items.filter((i) => i.status === 'done' && i.result);
          if (!doneItems.length) return;

          let totalBefore = 0, totalAfter = 0;
          for (const item of doneItems) {
            totalBefore += item.wp.file.size;
            totalAfter += item.result!.blob.size;
            const r = item.result!;
            const card = el('div', 'ftb-batch-card');
            const thumb = el('img', 'ftb-batch-thumb') as HTMLImageElement;
            thumb.src = URL.createObjectURL(r.blob);
            const info = el('div', 'ftb-batch-info');
            const displayName = item.wp.relativePath || item.wp.file.name;
            info.append(
              el('div', 'ftb-batch-name', displayName),
              el('div', 'ftb-batch-sizes', `${formatBytes(item.wp.file.size)} → ${formatBytes(r.blob.size)} · ${r.width}×${r.height}`),
            );
            const pct = ((1 - r.blob.size / item.wp.file.size) * 100).toFixed(1);
            const saved = r.blob.size < item.wp.file.size;
            const badge = el('span', 'ftb-batch-badge', saved ? `-${pct}%` : `+${pct}%`);
            badge.style.color = saved ? 'var(--success)' : 'var(--danger)';
            info.append(badge);

            const ext = r.format.startsWith('image/png') ? 'png' : r.format === 'image/webp' ? 'webp' : 'jpg';
            const dlBtn = button('下载', () => {
              downloadBlob(r.blob, item.wp.file.name.replace(/\.\w+$/, '') + '-compressed.' + ext);
            }, 'ghost');
            dlBtn.classList.add('ftb-batch-dl');
            card.append(thumb, info, dlBtn);
            resultGrid.append(card);
          }

          const savedPct = ((1 - totalAfter / totalBefore) * 100).toFixed(1);
          const summary = el('div', 'ftb-batch-summary');
          summary.append(el('span', undefined, `${doneItems.length} 张 · ${formatBytes(totalBefore)} → ${formatBytes(totalAfter)} · ${totalAfter < totalBefore ? '节省 ' + savedPct + '%' : '增大 ' + savedPct + '%'}`));

          const zipBtn = button('📥 全部下载 ZIP（含文件夹结构）', async () => {
            const zip = new JSZip();
            for (const item of doneItems) {
              const r = item.result!;
              const ext = r.format.startsWith('image/png') ? 'png' : r.format === 'image/webp' ? 'webp' : 'jpg';
              const fname = item.wp.file.name.replace(/\.\w+$/, '') + '-compressed.' + ext;
              const zipPath = item.wp.relativePath
                ? item.wp.relativePath.replace(/\/[^/]+$/, '') + '/' + fname
                : fname;
              zip.file(zipPath, r.blob);
            }
            const zipBlob = await zip.generateAsync({ type: 'blob' });
            downloadBlob(zipBlob, 'compressed-images.zip');
            toastSuccess('已打包下载（含文件夹结构）');
          });
          actions.append(summary, zipBtn);
        }

        // ---- 组装 ----
        const bar = el('div', 'ftb-toolbar');
        bar.append(
          Object.assign(el('label', undefined, '质量'), { style: 'font-size:13px;margin-right:4px' }),
          qualityInp,
          pngModeLabel,
          pngModeSel,
          sizeWrap,
          maxWidthInp,
        );

        layout.inputArea.append(bar, drop.container, listWrap, btnBar);
        layout.outputArea.append(resultGrid, actions);
        ctx.container.append(layout.container);
      },
    } satisfies ToolInstance;
  },
};

export default tool;
