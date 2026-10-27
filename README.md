# dramastudio

短剧批量视频生成平台——Web可视化界面，批量导入素材和Prompt，自动排队调用MiniMax API生成视频。

## 特性

- **Web界面**：浏览器操作，上传首帧图 + 填Prompt，或JSON批量导入
- **任务队列**：自动排队 → 后台生成 → 实时状态刷新（3秒轮询）
- **批量导入**：一次最多50条，JSON格式一键入队
- **视频预览**：生成完成直接在浏览器播放
- **模拟模式**：`DS_DRY_RUN=true` 不调API不烧额度，全流程可体验
- **单二进制**：纯Go + SQLite + 嵌入式Web UI，零外部依赖

## 快速开始

```bash
go build -o dramastudio ./cmd/dramastudio

# 模拟模式（先体验界面，不花一分钱）
DS_DRY_RUN=true ./dramastudio serve

# 真实模式（消耗MiniMax额度）
export DS_API_KEY=***
./dramastudio serve
# 浏览器打开 http://localhost:9090
```

## 批量导入格式

```json
[
  {"prompt": "镜头推近：年轻男人坐起喘气", "image_path": "uploads/frame1.png"},
  {"prompt": "镜头拉远：走廊全景"},
  {"prompt": "特写：手握手机，屏幕发光"}
]
```

`image_path` 可选——有则图生视频，无则纯文生视频。

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/tasks | 任务列表 |
| POST | /api/tasks | 创建单个 `{prompt, image_path}` |
| POST | /api/tasks/batch | 批量创建（JSON数组，≤50条） |
| DELETE | /api/tasks/{id} | 删除任务 |
| GET | /api/stats | 统计（pending/running/done/failed） |
| POST | /api/upload | 上传首帧图（multipart） |

## 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| DS_API_KEY | — | MiniMax密钥（真实模式必填） |
| DS_DRY_RUN | false | true=模拟模式 |
| DS_LISTEN | :9090 | 监听地址 |
| DS_VIDEO_MODEL | MiniMax-Hailuo-2.3 | 视频模型 |
| DS_DB | data/dramastudio.db | SQLite路径 |
| DS_DATA | data | 文件存储目录 |

## 与 dramapipe 的关系

- **dramapipe** = 命令行自动化（小说→剧本→分镜→视频→配音→成片）
- **dramastudio** = 可视化批量生产工具（准备好素材和prompt，它管批量生成）

两者互补：dramapipe的制作包产出分镜prompt → 导入dramastudio批量出视频。

## License

MIT
