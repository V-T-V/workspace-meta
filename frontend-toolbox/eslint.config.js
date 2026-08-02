// ESLint 9 flat config —— 前端工具箱（纯前端 DOM/Canvas 项目）。
// 与工作区兄弟项目（poetry-garden / superpower-system）保持一致的规则集。
import js from '@eslint/js';
import tseslint from 'typescript-eslint';
import prettierConfig from 'eslint-config-prettier';

export default tseslint.config(
  {
    ignores: ['node_modules/', 'dist/', '.git/', '*.log'],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  prettierConfig,
  {
    languageOptions: {
      ecmaVersion: 2023,
      sourceType: 'module',
      globals: {
        process: 'readonly',
        console: 'readonly',
        setTimeout: 'readonly',
        clearTimeout: 'readonly',
        setInterval: 'readonly',
        clearInterval: 'readonly',
        requestAnimationFrame: 'readonly',
        cancelAnimationFrame: 'readonly',
        queueMicrotask: 'readonly',
        TextEncoder: 'readonly',
        TextDecoder: 'readonly',
      },
    },
    rules: {
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
      '@typescript-eslint/no-explicit-any': 'off',
      'no-console': 'off',
    },
  },
  {
    files: ['src/**/*.ts'],
    languageOptions: {
      globals: {
        // DOM
        document: 'readonly',
        window: 'readonly',
        HTMLElement: 'readonly',
        HTMLCanvasElement: 'readonly',
        HTMLDivElement: 'readonly',
        HTMLButtonElement: 'readonly',
        HTMLInputElement: 'readonly',
        HTMLTextAreaElement: 'readonly',
        HTMLSelectElement: 'readonly',
        HTMLAnchorElement: 'readonly',
        HTMLImageElement: 'readonly',
        HTMLLabelElement: 'readonly',
        Element: 'readonly',
        Node: 'readonly',
        Event: 'readonly',
        MouseEvent: 'readonly',
        TouchEvent: 'readonly',
        KeyboardEvent: 'readonly',
        InputEvent: 'readonly',
        DragEvent: 'readonly',
        ClipboardEvent: 'readonly',
        CustomEvent: 'readonly',
        MutationObserver: 'readonly',
        IntersectionObserver: 'readonly',
        ResizeObserver: 'readonly',
        localStorage: 'readonly',
        fetch: 'readonly',
        location: 'readonly',
        history: 'readonly',
        crypto: 'readonly',
        SubtleCrypto: 'readonly',
        CryptoKey: 'readonly',
        URL: 'readonly',
        URLSearchParams: 'readonly',
        FileReader: 'readonly',
        Blob: 'readonly',
        File: 'readonly',
        FormData: 'readonly',
        Image: 'readonly',
        ImageData: 'readonly',
        // Canvas 2D
        CanvasRenderingContext2D: 'readonly',
        CanvasGradient: 'readonly',
        OffscreenCanvas: 'readonly',
      },
    },
  },
  {
    // 测试文件放宽：允许 import.meta.glob、测试 API
    files: ['test/**/*.ts'],
    languageOptions: {
      globals: {
        test: 'readonly',
        describe: 'readonly',
        it: 'readonly',
        before: 'readonly',
        after: 'readonly',
        beforeEach: 'readonly',
        afterEach: 'readonly',
        assert: 'readonly',
      },
    },
  },
);
