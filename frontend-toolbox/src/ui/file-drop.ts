// =============================================================================
// 文件拖拽 / 选择区 —— 支持点击选择 + 拖入放入，可限制类型与数量
// 支持文件夹：点击时 webkitdirectory 选文件夹，拖入时递归遍历子目录。
// v2：支持 onFilesWithPath 回调，保留文件夹内相对路径。
// =============================================================================

export interface FileDropOptions {
  accept?: string;
  multiple?: boolean;
  directory?: boolean;
  hint?: string;
  /** 选择/拖入回调（不含路径信息）。 */
  onFiles?: (files: File[]) => void;
  /** 选择/拖入回调（含相对路径，用于保留文件夹结构）。 */
  onFilesWithPath?: (files: FileWithPath[]) => void;
}

export interface FileWithPath {
  file: File;
  /** 相对于拖入文件夹根目录的路径，如 "sub/img.png"。单选文件时为空串。 */
  relativePath: string;
}

/** 从 DataTransfer 递归收集文件 + 相对路径。 */
async function collectFilesWithPaths(dt: DataTransfer): Promise<FileWithPath[]> {
  const result: FileWithPath[] = [];
  const items = dt.items;
  if (!items || items.length === 0) {
    return Array.from(dt.files).map((f) => ({
      file: f,
      relativePath: (f as File & { webkitRelativePath?: string }).webkitRelativePath ?? '',
    }));
  }

  const entries: Array<{ entry: FileSystemEntry; path: string }> = [];
  for (let i = 0; i < items.length; i++) {
    const entry = items[i]?.webkitGetAsEntry?.();
    if (entry) entries.push({ entry, path: '' });
  }

  if (entries.length > 0) {
    await traverseWithPath(entries, result);
    return result;
  }

  return Array.from(dt.files).map((f) => ({
    file: f,
    relativePath: (f as File & { webkitRelativePath?: string }).webkitRelativePath ?? '',
  }));
}

async function traverseWithPath(
  entries: Array<{ entry: FileSystemEntry; path: string }>,
  result: FileWithPath[],
): Promise<void> {
  for (const { entry, path } of entries) {
    if (entry.isFile) {
      const file = await new Promise<File>((resolve, reject) => {
        (entry as FileSystemFileEntry).file(resolve, reject);
      });
      result.push({ file, relativePath: path });
    } else if (entry.isDirectory) {
      const reader = (entry as FileSystemDirectoryEntry).createReader();
      const dirName = entry.name;
      const subPath = path ? path + '/' + dirName : dirName;
      let batch: FileSystemEntry[];
      do {
        batch = await new Promise<FileSystemEntry[]>((resolve) =>
          reader.readEntries(resolve),
        );
        await traverseWithPath(
          batch.map((e) => ({ entry: e, path: subPath })),
          result,
        );
      } while (batch.length > 0);
    }
  }
}

/** 创建文件拖拽区。 */
export function createFileDrop(opts: FileDropOptions): {
  container: HTMLElement;
  reset: () => void;
} {
  const { accept, multiple = false, directory = false, hint = '点击选择或拖拽文件到此处', onFiles, onFilesWithPath } = opts;
  const container = document.createElement('div');
  container.className = 'ftb-drop';
  container.tabIndex = 0;
  container.setAttribute('role', 'button');
  container.setAttribute('aria-label', '选择文件');

  const icon = document.createElement('div');
  icon.className = 'ftb-drop-icon';
  icon.textContent = '📁';

  const text = document.createElement('div');
  text.className = 'ftb-drop-text';
  text.textContent = hint;

  const sub = document.createElement('div');
  sub.className = 'ftb-drop-sub';

  container.append(icon, text, sub);

  const input = document.createElement('input');
  input.type = 'file';
  input.style.display = 'none';
  if (accept) input.accept = accept;
  if (multiple) input.multiple = true;
  if (directory) {
    input.webkitdirectory = true;
  }
  container.append(input);

  container.addEventListener('click', () => input.click());
  container.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); input.click(); }
  });

  input.addEventListener('change', () => {
    const rawFiles = Array.from(input.files ?? []);
    if (!rawFiles.length) return;
    if (onFilesWithPath) {
      const items: FileWithPath[] = rawFiles.map((f) => ({
        file: f,
        relativePath: (f as File & { webkitRelativePath?: string }).webkitRelativePath ?? '',
      }));
      onFilesWithPath(items);
    } else if (onFiles) {
      onFiles(rawFiles);
    }
    input.value = '';
  });

  const onDragOver = (e: DragEvent): void => { e.preventDefault(); container.classList.add('ftb-drop--over'); };
  const onDragLeave = (): void => container.classList.remove('ftb-drop--over');
  const onDrop = async (e: DragEvent): Promise<void> => {
    e.preventDefault();
    container.classList.remove('ftb-drop--over');
    if (onFilesWithPath) {
      const items = await collectFilesWithPaths(e.dataTransfer ?? new DataTransfer());
      if (items.length) onFilesWithPath(items);
    } else if (onFiles) {
      // 旧接口：收集文件（无路径）
      const dt = e.dataTransfer ?? new DataTransfer();
      const files: File[] = [];
      const items = dt.items;
      if (items && items.length) {
        const entries: FileSystemEntry[] = [];
        for (let i = 0; i < items.length; i++) {
          const entry = items[i]?.webkitGetAsEntry?.();
          if (entry) entries.push(entry);
        }
        if (entries.length) {
          // 复用原有的文件收集（无路径版本）
          const collectFiles = async (ents: FileSystemEntry[], out: File[]): Promise<void> => {
            for (const e of ents) {
              if (e.isFile) {
                out.push(await new Promise<File>((resolve, reject) => {
                  (e as FileSystemFileEntry).file(resolve, reject);
                }));
              } else if (e.isDirectory) {
                const reader = (e as FileSystemDirectoryEntry).createReader();
                let batch: FileSystemEntry[];
                do {
                  batch = await new Promise<FileSystemEntry[]>((r) => reader.readEntries(r));
                  await collectFiles(batch, out);
                } while (batch.length > 0);
              }
            }
          };
          await collectFiles(entries, files);
        }
      }
      if (!files.length) files.push(...Array.from(dt.files));
      if (files.length) onFiles(multiple ? files : files.slice(0, 1));
    }
  };
  container.addEventListener('dragover', onDragOver);
  container.addEventListener('dragleave', onDragLeave);
  container.addEventListener('drop', onDrop);

  return {
    container,
    reset(): void { sub.textContent = ''; },
  };
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`;
}
