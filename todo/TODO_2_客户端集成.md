# Desktop 客户端改造 TODO（Wails + React/Tailwind + shadcn/ui）

目标：将现有 Go 单体服务打包为桌面客户端（macOS/Windows），前端实现现代化（shadcn 风格、少自定义样式），在不大幅侵入后端的前提下分阶段渐进迁移。优先路线：Wails + React/Vite/Tailwind + shadcn（Phase B），稳定后可选用 Wails bindings（Phase C）去掉本地端口与 HTTP 调用。

---

## 里程碑与总览

1) Phase 0：准备与对齐（0.5 天）  
2) Phase 1：Wails 容器落地（1 天）  
3) Phase 2：前端基座搭建（1 天）  
4) Phase 3：页面迁移（Settings → Endpoints → Logs/Inspector）（5–7 天）  
5) Phase 4：性能与工程化（1 天）  
6) Phase 5（可选）：Wails bindings 深度收口（2–3 天）  
7) Phase 6：打包与发布（0.5–1 天）

所有阶段均要求：可回滚、可阶段交付；每完成一页的迁移即可去除相应 Bootstrap/JS 资源。

---

## Phase 0：准备与对齐（0.5 天）

- 明确路线：先 Phase B（React 前端经 HTTP 调用现有 Go 服务），再视情况进入 Phase C（bindings）。
- 目录规划：
  - 后端沿用当前仓库结构（`internal/*`, `main.go`）。
  - 新增 `wails-app/` 作为桌面工程根；其下 `frontend/` 放置 React+Vite 代码。
- 视觉规范：
  - 统一字体（系统字体/Inter）、图标（lucide）、色板/圆角/阴影等令牌由 Tailwind 主题管理。
  - 减少自定义 CSS，优先使用 shadcn 组件与 Tailwind 工具类。

验收：确认目录结构与技术路线，无代码改动风险。

---

## Phase 1：Wails 容器落地（1 天）

1. 安装 Wails 工具链：
   - `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
   - `wails doctor`（检查依赖：Xcode/VS Build Tools、WebView2 等）

2. 初始化 Wails 工程（与现有仓库共存）：
   - 在项目根创建 `wails-app/` 目录。
   - 生成基础配置 `wails.json`（指向前端目录和启动命令）。示例：
     - `frontend: "wails-app/frontend"`
     - `devServer: { command: "npm run dev", url: "http://localhost:5173" }`
     - `build: { command: "npm run build", outputPath: "dist" }`

3. 后端启动策略（Phase B）：
   - 应用启动时在 `127.0.0.1:0` 启动现有 Go 服务（随机端口），记录端口。
   - 窗口加载 React 构建产物（或 dev server），React 通过 `baseURL = http://127.0.0.1:<port>` 访问后端。
   - 设定严格 CSP，禁用外部导航，资源走本地。

4. 本地开发：
   - 后端：沿用 `go run ./`（或在 Wails 进程内启动）。
   - 前端：`npm run dev`（Vite），Wails dev 模式加载该 URL。

验收：`wails dev` 可启动窗口，前后端联通（可先加载原 SSR 页面验证管道畅通）。

---

## Phase 2：前端基座（1 天）

1. 初始化前端（React + Vite + TS）：
   - 在 `wails-app/` 下：`npm create vite@latest frontend -- --template react-ts`
   - 进入 `wails-app/frontend`：`npm i`。

2. Tailwind 与 shadcn：
   - `npm i -D tailwindcss postcss autoprefixer`
   - `npx tailwindcss init -p`
   - Tailwind `content` 指向 `index.html` 与 `src/**/*.{ts,tsx}`。
   - `npx shadcn@latest init`（选择样式方案，如 new-york），启用 `tailwindcss-animate`。
   - 安装基础组件：`npx shadcn@latest add button card input select dialog table toast tabs accordion form scrollbar`。
   - 图标：`npm i lucide-react`；字体：按需引入 Inter 或系统字体。

3. 项目基架：
   - 建立 `src/app/layout.tsx`（全局 Layout/Header/Sidebar/Content/Toaster）。
   - 建立 `src/shared/api.ts`（封装 fetch/axios，注入 `baseURL` 与错误处理）。
   - 路由：`react-router-dom` 定义 `/settings`, `/endpoints`, `/logs` 等。

4. MCP 支持（shadcn MCP 文档）：
   - 按 https://ui.shadcn.com/docs/mcp 步骤接入（安装依赖、配置 providers、在需要的页面挂载组件）。
   - 将 MCP 交互与现有后端接口解耦（仅做前端能力增强）。

验收：应用能渲染 Layout，按钮/对话框/通知等 shadcn 组件正常；主题令牌生效。

---

## Phase 3：页面迁移（逐页替换，可随时发布）

通用要求：迁移一页就删除对应 Bootstrap/JS 资源；行为一致、接口不变。

### 3.1 Settings（1 天）
步骤：
1. 对照 `web/templates/settings.html` 与 `web/static/settings.js`，梳理字段与接口（读取/保存、校验）。
2. 用 shadcn Form/Input/Select/Switch/Card 重建设置表单；复用现有校验规则与提示文案。
3. 将保存/重置按钮改为 shadcn Button + Toaster 成功/错误提示。
4. 交互与接口保持一致：
   - 请求/响应头与当前一致；
   - 超时字段沿用 Go duration 表达，前端仅做基本校验与提示；
   - 国际化占位（沿用 `web/locales` 资源，后续再抽取）。
验收：设置读写成功、必填/格式提示准确、UI 一致并更现代。

### 3.2 Endpoints（2–3 天）
步骤：
1. 列表页：
   - 用 shadcn Table（基于 TanStack Table）实现优先级、名称、URL、代理、标签、配置、状态、操作列；
   - 拖拽排序使用 `@dnd-kit/sortable` 替代 SortableJS；
   - 顶部工具栏（重置/测试/向导/添加）使用 shadcn Button + Dropdown/Menu；
   - 批量测试按钮对接原 `/api/endpoints/test-all`。
2. 弹窗/向导：
   - Dialog/Drawer/Steps 组件重做 `endpoint-modal.html`、`endpoint-wizard-modal.html`；
   - 参数联动、校验与原逻辑一致；
   - OpenAI 自适应偏好显示与编辑（responses/chat_completions/auto）。
3. 高级配置：
   - Model rewrite、黑名单策略、并发竞速等以 Accordion + Form 呈现；
   - 帮助提示统一用 Tooltip/Popover；
4. 清理：移除 `web/static/vendor/sortablejs/*`、`endpoints-*.js/css` 中已替代的逻辑与样式。
验收：端点增删改查、拖拽排序、批量测试、弹窗/向导全通过；无回归。

### 3.3 Logs/Inspector（2 天）
步骤：
1. 页布局：Tabs/Accordion 分区（概览、请求/响应、工具调用、系统提示、对比等）。
2. 代码/长文档区域使用 ScrollArea + Code 样式（等宽字体、折行/复制）。
3. 流式/大数据优化：惰性渲染、虚拟滚动（必要时）；保持与现有解析器一致。
4. 将图标统一为 lucide，动画用 `tailwindcss-animate` 或 Motion One。
验收：日志加载、筛选、查看、对比稳定，长日志不卡顿。

---

## Phase 4：性能与工程化（1 天）

- 构建优化：Vite splitChunks、`@rollup/plugin-visualizer` 检查体积、按需引入 shadcn 组件、移除未用依赖。
- 资源优化：关键 CSS preload、图片/字体内联阈值、长缓存与文件名哈希。
- 安全：严格 CSP、禁用外部导航、只允许本地 API；错误上报仅本地存储/文件。

验收：首屏时间、包体、内存占用达标（自定目标，如启动 < 1.5s，包体 < 70MB）。

---

## Phase 5（可选）：Wails bindings 深度收口（2–3 天）

目标：消除本地 HTTP 端口与 CORS/鉴权负担，提升调用速度与一致性。

步骤：
1. 选取热路径（设置读写、端点列表/Test、日志读取）逐个新增 Go bindings。
2. `wails generate` 生成 TS 声明；前端 API 客户端改用 bindings；保留 HTTP 为 feature flag 回退。
3. 文件对话框/通知等改用 Wails API；
4. 端到端测试：确保无回归后关闭 HTTP 分支。

验收：应用仅依赖 bindings，无本地端口；关键路径速度更快。

---

## Phase 6：打包与发布（0.5–1 天）

- macOS：`wails build -platform darwin/universal` 生成 `.app` 与 `.dmg`；
- Windows：`wails build -platform windows/amd64` 生成安装包（msi/nsis）；
- 可选签名与图标资源；
- 启动自检：后端起活探针，端口失败自动重试，崩溃日志采集。

验收：安装/启动/卸载流程通畅，无安全提示（或在白名单策略内）。

---

## 任务清单（可勾选）

- [ ] Phase 0：目录/路线确认；视觉令牌草案
- [ ] Phase 1：Wails 容器与 `wails.json`；本地端口策略与 CSP
- [ ] Phase 2：Vite + React + Tailwind + shadcn 初始化；Layout/路由/API 客户端
- [ ] Phase 3.1：Settings 迁移；删除对应 Bootstrap 资源
- [ ] Phase 3.2：Endpoints 列表/弹窗/向导迁移；去除 SortableJS；批量测试联通
- [ ] Phase 3.3：Logs/Inspector 迁移；长日志性能验证
- [ ] Phase 4：打包体积/首屏/内存优化；CSP 硬化
- [ ] Phase 5（可选）：Bindings 收口；移除本地端口
- [ ] Phase 6：打包与签名；发布验证

---

## 验收标准（示例）

- 桌面化：双平台安装包可用；首次启动 < 1.5s；二次启动 < 1.0s。
- 视觉一致：shadcn 主题令牌统一字号/间距/圆角；无额外定制 CSS（< 200 行）。
- 功能等价：Settings/Endpoints/Logs 行为与当前一致；批量测试、拖拽排序稳定。
- 安全与稳定：CSP 严格；无外部请求；崩溃日志可定位。

---

## 回退策略

- 每个页面迁移完成前，旧 SSR 页面仍保留路由入口，可随时切回。
- Bindings 开启前保留 HTTP 实现（feature flag），随时回滚。
- 变更均分支开发，预发布验收后合入主分支。

---

## 参考命令速查（汇总）

```bash
# Wails 工具
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor
wails dev
wails build -platform darwin/universal
wails build -platform windows/amd64

# 前端脚手架
cd wails-app
npm create vite@latest frontend -- --template react-ts
cd frontend && npm i
npm i -D tailwindcss postcss autoprefixer tailwindcss-animate
npx tailwindcss init -p
npx shadcn@latest init
npx shadcn@latest add button card input select dialog table toast tabs accordion form scrollbar
npm i lucide-react
```
