# Movie Narrator Rebuild — Project Context & Development Plan

> Tài liệu tổng hợp dùng làm context cho Codex/AI coding agent khi xây dựng hệ thống mới dựa trên repo:
>
> https://github.com/zcbacxc/movie-narrator.git
>
> Mục tiêu: không chỉ fork và vá dần, mà xây một hệ thống video AI có kiến trúc riêng, có thể tách server/client, mở rộng lâu dài, nhưng vẫn tận dụng những phần tốt của Movie Narrator.

---

# 1. Mục tiêu dự án

Dùng Movie Narrator làm **reference implementation + Engine V1**, sau đó xây một nền tảng mới theo hướng:

- Client và server tách riêng.
- Movie Narrator chỉ là một engine xử lý media/AI.
- Backend của sản phẩm quản lý user/project/job/asset/database/storage.
- Pipeline có thể pause/resume.
- Có checkpoint và partial execution.
- Có timeline editor.
- Có script editor.
- Có subtitle editor.
- Có voice editor.
- Có scene editor.
- Có character recognition.
- Có VLM/LLM/TTS/ASR provider abstraction.
- Có thể scale worker CPU/GPU riêng.
- Có thể xuất nhiều format từ cùng một project.
- Có thể mở rộng thành AI Video Production Engine thay vì chỉ Movie Recap Generator.

---

# 2. Repo gốc đang làm gì?

Movie Narrator là một pipeline tạo video recap/narration từ video nguồn.

Pipeline chính bao gồm các stage dạng:

```text
resolve_video
prepare_assets
research_plot
generate_script
export_script_md
generate_voice
align_audio
detect_scenes
match_clips
mix_bgm
translate_subtitles
generate_subtitle
run_qa_gate
render_video
validate_deliverable
export_clips
```

Chức năng chính:

- Research nội dung phim.
- Sinh script bằng LLM.
- Tạo voice-over bằng TTS.
- Alignment bằng Whisper/WhisperX/faster-whisper.
- Scene detection.
- Scene/script semantic matching.
- Subtitle.
- Background music.
- Render video.
- QA đầu ra.
- REST API.
- Async queue.
- Worker.
- Plugin system.
- Docker/Compose.
- Distributed/remote execution.
- Artifact storage.

---

# 3. Công nghệ / engine chính

## Core

- Python
- Typer CLI
- Pydantic
- YAML / dotenv

## AI

- OpenAI-compatible LLM API
- Ollama
- qwen2.5:7b mặc định theo cấu hình local
- Có thể dùng provider tương thích OpenAI API
- VLM provider
- sentence-transformers

## Speech / Audio

- Edge-TTS
- OpenAI TTS
- MiMo TTS
- WhisperX
- faster-whisper
- FunASR
- pydub

## Video

- FFmpeg
- MoviePy 2.x
- PySceneDetect
- Pillow

## Server / Worker

- REST server
- ThreadingHTTPServer trong implementation hiện tại
- LocalTaskQueue
- RemoteTaskQueue
- scheduler
- artifact store

## Storage

- local filesystem
- SQLite
- S3-compatible
- MinIO
- boto3

## Deployment

- Docker
- Docker Compose
- NVIDIA CUDA worker image

---

# 4. Security review tổng hợp

## Kết luận

Không thấy dấu hiệu rõ ràng cho thấy repo là malware/virus.

Tuy nhiên, không nên xem đây là hệ thống production-secure nếu expose trực tiếp ra Internet.

## Điểm tốt

- API mặc định bind localhost.
- Public bind cần API key hoặc explicit insecure mode.
- API key comparison dùng hmac.compare_digest.
- Có giới hạn request size.
- Có kiểm soát artifact path traversal.
- YAML dùng safe_load.
- SQLite query parameterized.
- Docker runtime chạy non-root.
- CI chạy:
  - tests
  - Ruff
  - mypy
  - Bandit
  - pip-audit
- Có test matrix nhiều phiên bản Python.

## Rủi ro đáng chú ý

### 4.1 Pillow advisory

Repo hiện dùng Pillow 11.x vì MoviePy constraint.

CI có ignore một số advisory của Pillow.

=> Không phải malware, nhưng là dependency security debt.

Cần:

- theo dõi MoviePy upstream;
- upgrade Pillow khi dependency cho phép;
- sandbox media processing.

### 4.2 Plugin system

Plugin Python được load bằng entry points.

Bản chất:

```text
third-party plugin = arbitrary Python code
```

Plugin có thể:

- đọc filesystem;
- đọc environment variables;
- lấy API key;
- gọi network;
- execute process.

Khuyến nghị:

- plugin allowlist;
- không auto-enable plugin lạ;
- server production mặc định disable third-party plugin;
- plugin chạy subprocess/container riêng nếu cần.

### 4.3 REST API không tự cung cấp TLS

API key qua HTTP không đủ an toàn nếu chạy Internet.

Production nên:

```text
Internet
   ↓
Caddy / Nginx / Traefik HTTPS
   ↓
Movie Narrator API localhost
```

hoặc:

- WireGuard
- Tailscale
- VPN

### 4.4 Custom FFmpeg executable

Có thể cấu hình executable FFmpeg qua biến môi trường.

Không nên trust `.env` từ project không rõ nguồn gốc.

Khuyến nghị:

- không cho project-level config override executable trong production;
- hoặc cần explicit allow;
- hoặc executable allowlist.

### 4.5 Bandit subprocess blind spot

FFmpeg-heavy code dễ có nhiều subprocess.

Không nên skip subprocess security checks toàn cục.

Khuyến nghị:

- wrapper duy nhất cho subprocess;
- cấm shell=True;
- review từng call;
- CI rule riêng.

### 4.6 Media parsing

FFmpeg/Pillow/media codec là attack surface lớn nếu nhận file từ Internet.

Production architecture nên:

```text
Public API
   ↓
Queue
   ↓
Disposable Media Worker
   ├─ non-root
   ├─ no secrets
   ├─ limited filesystem
   ├─ limited outbound network
   ├─ CPU quota
   ├─ RAM quota
   └─ time limit
```

### 4.7 MinIO

Không expose MinIO ra public với default credential.

Pin image version thay vì dùng latest cho production.

### 4.8 Edge-TTS

Edge-TTS là unofficial/reverse-engineered interface.

Phù hợp:

- local
- test
- personal

Commercial production nên cân nhắc provider chính thức.

---

# 5. Có nên tách server/client?

Có.

Repo hiện đã có REST daemon và remote queue nên việc tách là tự nhiên.

Không nên để browser gọi trực tiếp Movie Narrator API.

Không nên:

```text
Browser
   ↓
X-API-Key
   ↓
Movie Narrator
```

vì key có thể bị lấy từ frontend.

Nên:

```text
Browser
   ↓
Your Backend API
   ↓
Movie Narrator Engine
```

---

# 6. Kiến trúc mục tiêu

```text
                         USER
                          │
                          ▼
                  ┌──────────────┐
                  │   WEB APP    │
                  │ Next.js/React│
                  └──────┬───────┘
                         │ HTTPS
                         ▼
                  ┌──────────────┐
                  │    FastAPI   │
                  │  Product API │
                  └──────┬───────┘
                         │
           ┌─────────────┼─────────────┐
           │             │             │
           ▼             ▼             ▼
      PostgreSQL        Redis        S3/MinIO
           │             │             │
           │          Job Queue         │
           │             │             │
           │             ▼             │
           │      ┌──────────────┐      │
           └─────►│ Engine Worker│◄─────┘
                  │              │
                  │ Movie        │
                  │ Narrator V2  │
                  └──────┬───────┘
                         │
              ┌──────────┼───────────┐
              ▼          ▼           ▼
             LLM        VLM         TTS
              │
             ASR
              │
        Scene Intelligence
              │
           Timeline
              │
            FFmpeg
              │
              ▼
            Result
```

---

# 7. Kiến trúc repo đề xuất

```text
your-project/

├── apps/
│   ├── web/
│   │   └── Next.js / React
│   │
│   └── api/
│       └── FastAPI
│
├── services/
│   ├── engine/
│   │   ├── core/
│   │   ├── pipeline/
│   │   ├── media/
│   │   ├── ai/
│   │   ├── tts/
│   │   ├── vision/
│   │   ├── speech/
│   │   ├── rendering/
│   │   └── providers/
│   │
│   └── worker/
│
├── packages/
│   ├── contracts/
│   ├── sdk/
│   └── shared/
│
├── infra/
│   ├── docker/
│   ├── postgres/
│   ├── redis/
│   ├── minio/
│   └── reverse-proxy/
│
├── tests/
│   ├── unit/
│   ├── integration/
│   └── e2e/
│
├── docs/
│
└── tools/
```

---

# 8. Không nên rewrite toàn bộ ngay

Không làm:

```text
DELETE src/movie_narrator
→ viết lại mọi thứ
```

Nên dùng Strangler Pattern.

Ban đầu:

```text
New Product API
      │
      ▼
VideoEngine Interface
      │
      ▼
LegacyMovieNarratorAdapter
      │
      ▼
Movie Narrator V1
```

Sau đó từng module V1 được thay bằng V2.

Ví dụ:

```text
Phase A:
NEW Script Engine
OLD TTS
OLD Scene
OLD Matcher
OLD Render

Phase B:
NEW Script
NEW TTS
OLD Scene
OLD Matcher
OLD Render

Phase C:
NEW Script
NEW TTS
NEW Scene
NEW Matcher
OLD Render

Phase D:
ALL V2
```

---

# 9. Engine abstraction

Backend sản phẩm không nên phụ thuộc trực tiếp implementation.

Interface:

```python
class VideoEngine:
    async def create_job(self, request):
        ...

    async def start_job(self, job_id):
        ...

    async def cancel_job(self, job_id):
        ...

    async def get_status(self, job_id):
        ...

    async def get_artifacts(self, job_id):
        ...
```

Implementation:

```python
class MovieNarratorEngine(VideoEngine):
    ...
```

Sau này có thể thêm:

```text
VideoEngine
    ├── MovieNarratorEngine
    ├── ShortsEngine
    ├── RemotionEngine
    ├── ComfyUIEngine
    ├── ImageToVideoEngine
    └── CustomRenderEngine
```

---

# 10. Pipeline architecture mới

Không nên dùng pipeline cứng:

```python
step1()
step2()
step3()
```

Nên mỗi step là node độc lập:

```python
class PipelineNode:
    name: str

    async def execute(self, context):
        ...
```

Ví dụ node:

```text
ResearchNode
ScriptNode
ScriptReviewNode
TTSNode
AlignmentNode
SceneDetectionNode
CharacterDetectionNode
VLMCaptionNode
ClipMatchingNode
TimelineNode
SubtitleNode
AudioMixNode
RenderNode
QualityControlNode
```

---

# 11. JobContext

Dữ liệu pipeline nên đi qua object chung.

Ví dụ:

```python
class JobContext:
    job_id: str
    project_id: str

    input_asset: str

    research: object | None
    script: object | None
    narration: object | None

    scenes: list
    characters: list
    matches: list

    timeline: object | None
    subtitles: object | None

    artifacts: dict
```

Node chỉ đọc/ghi phần mình cần.

---

# 12. Checkpoint + partial execution

Đây là chức năng ưu tiên cao.

Cần hỗ trợ:

```text
run --until generate_script
```

và:

```text
run --from generate_voice
```

hoặc API:

```json
{
  "start_from": "generate_voice",
  "stop_after": "match_clips"
}
```

Mục tiêu:

```text
Research
   ↓
Script
   ↓
PAUSE
   ↓
Human Edit
   ↓
TTS
   ↓
Scene Matching
   ↓
PAUSE
   ↓
Human Edit
   ↓
Render
```

Có hai mode:

## Automatic

```text
Video
 ↓
AI Pipeline
 ↓
Final Video
```

## Studio

```text
Video
 ↓
AI Script
 ↓
Human Review
 ↓
Voice
 ↓
Human Review
 ↓
Timeline
 ↓
Human Review
 ↓
Render
```

---

# 13. Progress Event System

Không nên để frontend poll mù.

Engine nên emit event:

```python
stage_started(stage)
stage_progress(stage, percent)
stage_completed(stage)
stage_failed(stage, error)
```

Các reporter có thể là:

```text
ConsoleProgressReporter
RedisProgressReporter
WebSocketProgressReporter
NullProgressReporter
```

Frontend hiển thị:

```text
Researching plot        100%
Generating script       100%
Generating voice        100%
Detecting scenes         82%
Matching clips           41%
Rendering                 0%
```

---

# 14. Artifact model

Không truyền raw filesystem path khắp hệ thống.

Sai:

```text
/home/user/movie.mp4
```

Nên:

```json
{
  "id": "asset_01...",
  "type": "video",
  "storage": "local",
  "uri": "...",
  "duration": 7200,
  "width": 1920,
  "height": 1080
}
```

Storage backend:

```text
LocalStorage
S3Storage
MinIOStorage
```

Engine dùng Asset ID, không phụ thuộc filesystem thật.

---

# 15. Project model

Mỗi production là một Project.

```text
Project

├── Source Assets
├── Research
├── Scripts
│   ├── v1
│   ├── v2
│   └── final
├── Voice
├── Scenes
├── Characters
├── Timeline
├── Subtitles
├── Music
└── Renders
    ├── youtube.mp4
    ├── shorts.mp4
    └── tiktok.mp4
```

Mục tiêu là chuyển từ:

```text
command → output folder
```

sang:

```text
Project → editable state → render versions
```

---

# 16. Timeline là trung tâm

AI không nên render video trực tiếp.

Nên:

```text
AI
 ↓
Timeline Proposal
 ↓
Human / Automation Editor
 ↓
Timeline
 ↓
Renderer
```

Timeline:

```json
{
  "tracks": [
    {
      "type": "video",
      "clips": []
    },
    {
      "type": "narration",
      "clips": []
    },
    {
      "type": "music",
      "clips": []
    },
    {
      "type": "subtitle",
      "clips": []
    }
  ]
}
```

Điều này tạo nền cho editor kiểu CapCut/Premiere đơn giản.

---

# 17. Script editor

Workflow:

```text
Research
 ↓
Generate Script
 ↓
STOP
 ↓
User Edit
 ↓
Approve
 ↓
TTS
```

Không để engine phụ thuộc frontend.

Backend chỉ lưu version script.

---

# 18. Scene editor

AI tạo:

```text
matches.json
```

Frontend cho user sửa:

```text
Scene 12

AI:
00:41:20 → 00:41:29

User:
00:52:13 → 00:52:21
```

Render đọc edited timeline/matches.

---

# 19. Scene Intelligence V2

Scene không chỉ có start/end.

Nên có:

```json
{
  "id": "scene_314",
  "start": 1234.2,
  "end": 1241.8,
  "description": "...",
  "characters": [],
  "dialogue": "...",
  "emotion": "tense",
  "location": "warehouse",
  "actions": ["running", "shooting"],
  "visual_embedding": "...",
  "text_embedding": "...",
  "quality_score": 0.91
}
```

---

# 20. Character Engine

Pipeline:

```text
Movie
 ↓
Face detection
 ↓
Face tracking
 ↓
Face clustering
 ↓
Character Database
```

Character:

```text
Character #4
faces: 238
scenes: 45
name: Tony Stark
```

Khi script nói:

```text
Tony enters the laboratory
```

matcher có thể:

```text
Tony
 ↓
character_4
 ↓
find scenes containing character_4
```

thay vì chỉ semantic text search.

---

# 21. Multimodal scene matching

Đề xuất:

```text
Script Segment
     ↓
Text Embedding
     ├───────────────┐
     ▼               ▼
VLM captions     Visual embedding
                 CLIP / SigLIP
     │               │
     └──── Fusion Scorer ──────┐
                                ▼
                     Character scorer
                                │
                         Temporal scorer
                                │
                         Diversity score
                                │
                                ▼
                         Best Video Clip
```

Thêm:

- visual embeddings;
- face/character tracking;
- action detection;
- OCR;
- temporal context;
- anti-repeat penalty;
- clip diversity;
- visual quality score.

---

# 22. Script ↔ footage feedback loop

Một cải tiến quan trọng:

```text
Draft Script
    ↓
Scene Matching
    ↓
Coverage Score
    ↓
Không có footage phù hợp
    ↓
Rewrite Script Segment
    ↓
Re-match
```

Mục tiêu:

AI không chỉ là writer.

Nó trở thành editor-aware writer.

---

# 23. Provider abstraction

Không hard-code model provider.

## LLM

```python
class LLMProvider:
    async def generate(...):
        ...
```

Backend có thể:

```text
OpenAI
Ollama
vLLM
LM Studio
Qwen
GLM
SiliconFlow
...
```

## Vision

```python
class VisionProvider:
    async def describe_frame(...):
        ...

    async def describe_scene(...):
        ...
```

Có thể:

```text
OpenAI
Gemini
Qwen-VL
InternVL
MiniCPM-V
Ollama Vision
vLLM multimodal
```

## TTS

```text
EdgeTTS
OpenAI
MiMo
ElevenLabs
local TTS
custom
```

## ASR

```text
Whisper
WhisperX
faster-whisper
FunASR
custom
```

## Embedding

```text
sentence-transformers
CLIP
SigLIP
custom
```

---

# 24. Local/offline mode

Mục tiêu:

```text
Movie
 ↓
Local Whisper
 ↓
Local VLM
 ↓
Local LLM
 ↓
Local TTS
 ↓
FFmpeg
```

Không gửi media lên cloud.

Có lợi cho:

- privacy;
- batch processing;
- chi phí;
- development.

---

# 25. Audio Engine V2

Thêm:

- LUFS target;
- EBU R128;
- true peak protection;
- side-chain ducking;
- speech activity-aware ducking;
- beat detection;
- cut theo beat;
- SFX;
- prosody per sentence;
- pause/breath model;
- voice consistency QA.

---

# 26. Subtitle Engine V2

Ngoài SRT:

- word-by-word highlight;
- karaoke timing;
- active-word animation;
- smart line breaking;
- keyword emphasis;
- dynamic placement tránh mặt;
- style template;
- multilingual/bilingual subtitle.

---

# 27. Auto-reframe

16:9 → 9:16 không chỉ crop center.

Pipeline:

```text
Frame
 ↓
Face detection
 ↓
Subject detection
 ↓
Motion saliency
 ↓
Smooth tracking
 ↓
Dynamic Crop
 ↓
9:16
```

Output:

- YouTube
- Shorts
- TikTok
- Reels
- 1:1

---

# 28. Multi-output

Một project có thể xuất:

```text
YouTube 16:9
TikTok 9:16
Shorts 9:16
Facebook 1:1
30s teaser
60s recap
10min recap
```

Không chạy lại toàn pipeline.

Chỉ đổi timeline/render profile.

---

# 29. Workflow model

Movie recap chỉ là một workflow.

```text
workflows/

├── movie_recap
├── documentary
├── shorts
├── highlights
├── trailer
├── commentary
├── reaction
└── custom
```

---

# 30. Product features nên để ngoài engine

Không đưa vào core engine:

- users;
- subscriptions;
- billing;
- workspace;
- permissions;
- website;
- frontend;
- team;
- quota.

Backend sản phẩm quản lý:

```text
users
projects
assets
jobs
job_steps
templates
voices
characters
scripts
renders
subscriptions
api_keys
```

Engine chỉ nhận job.

---

# 31. Backend flow

```text
Client
 ↓
POST /projects
 ↓
Project created

Client
 ↓
Upload asset
 ↓
S3 / MinIO

Client
 ↓
POST /projects/{id}/jobs
 ↓
Backend creates Job
 ↓
Queue
 ↓
Engine Worker
 ↓
Artifacts
 ↓
Backend
 ↓
Client
```

---

# 32. Upload file lớn

Không nên:

```text
Browser → FastAPI → Worker
```

cho video 5–50 GB.

Nên:

```text
Browser
 ↓
Presigned URL
 ↓
S3 / MinIO
```

Backend chỉ nhận:

```text
asset upload completed
```

---

# 33. Worker architecture

Ban đầu:

```text
API
 ↓
Movie Narrator Worker
```

đủ.

Sau này:

```text
                 Job Queue
                     │
         ┌───────────┼───────────┐
         ▼           ▼           ▼
      AI Worker   ML Worker   Render Worker
         │           │           │
       LLM       Whisper/VLM   FFmpeg
```

Lý do:

- LLM: network-bound.
- Whisper/VLM: GPU-heavy.
- FFmpeg: CPU/GPU + disk I/O-heavy.

---

# 34. Security production mode

Nên có:

```bash
engine serve --production
```

Enforce:

```text
API key required
HTTPS/reverse proxy awareness
third-party plugins disabled
custom FFmpeg disabled
rate limiting
request quota
media size limit
media duration limit
secret redaction
sandbox worker
production storage
```

---

# 35. Arch Linux development setup

Arch rolling release thường dùng Python rất mới.

Movie Narrator ML stack không nên dựa vào Python hệ thống.

Khuyến nghị:

```text
uv
+
Python 3.13
+
project virtualenv
```

Ví dụ:

```bash
sudo pacman -S uv ffmpeg git

uv python install 3.13

uv venv --python 3.13

source .venv/bin/activate
```

Sau đó:

```bash
uv pip install -e ".[dev,full]"
```

Không cài dependency ML trực tiếp vào Python hệ thống Arch.

---

# 36. Chuyển repo thành repo riêng

Có thể:

```bash
git clone https://github.com/zcbacxc/movie-narrator.git
cd movie-narrator

git remote rename origin upstream

git remote add origin git@github.com:YOUR_USERNAME/YOUR_PROJECT.git

git push -u origin main
```

Remote:

```text
origin
  ↓
repo của bạn

upstream
  ↓
zcbacxc/movie-narrator
```

Tag baseline:

```bash
git tag upstream-v1.1.0
git push origin upstream-v1.1.0
```

Lưu ý license upstream là AGPL-3.0-or-later.

Không xóa attribution/license nếu tiếp tục sử dụng/modify code upstream.

---

# 37. Baseline branch

Trước khi sửa:

```bash
git checkout -b baseline/original-engine
```

Xác nhận:

```text
CLI chạy
Ollama chạy
TTS chạy
FFmpeg chạy
script chạy
voice chạy
scene detection chạy
matching chạy
render chạy
REST API chạy
tests pass
```

Sau đó không sửa branch baseline.

---

# 38. Git strategy

Giữ:

```text
main
```

ổn định.

Feature branches:

```text
feature/project-model
feature/job-engine
feature/timeline
feature/script-editor
feature/scene-index
feature/character-engine
feature/vlm-provider
feature/render-v2
feature/checkpoints
feature/progress-events
```

Refactor:

```text
refactor/pipeline-runtime
refactor/artifact-model
```

---

# 39. Không đổi namespace legacy ngay

Không đổi hàng loạt:

```text
movie_narrator
→ new_name
```

ngay từ đầu.

Việc đó tạo diff lớn không cần thiết và làm khó đối chiếu upstream.

Giữ:

```text
movie_narrator
```

làm legacy namespace.

Project mới có namespace riêng:

```text
your_engine
```

Adapter:

```text
your_engine
     ↓
movie_narrator
```

Khi V2 thay thế gần hết V1 mới remove namespace cũ.

---

# 40. Test strategy

Giữ regression test cho engine cũ.

Luôn đảm bảo:

```text
CLI create
REST task
pipeline stages
render
config loading
plugin loading
```

vẫn hoạt động trong thời gian migration.

Thêm:

```text
tests/
├── unit/
├── integration/
├── regression/
└── e2e/
```

---

# 41. Roadmap đề xuất

## Phase 0 — Freeze upstream

```text
clone repo
create own repo
add upstream remote
tag baseline
pin Python 3.13
environment reproducible
tests pass
sample video chạy được
```

Không thêm feature.

## Phase 1 — Skeleton V2

Tạo:

```text
apps/
services/
packages/
infra/
```

Có:

```text
FastAPI
PostgreSQL
Redis
MinIO
```

Engine vẫn gọi V1.

Mục tiêu:

```text
Browser
 ↓
API
 ↓
Job
 ↓
Movie Narrator V1
 ↓
Result
```

## Phase 2 — Domain Model

Xây:

```text
Project
Asset
Job
Pipeline
Node
Artifact
Timeline
Scene
Script
Character
Render
```

## Phase 3 — Engine V2

Viết:

```text
pipeline runtime
checkpoint
resume
events
progress
artifact management
provider system
```

Module nào chưa viết thì gọi V1 qua adapter.

## Phase 4 — Studio

UI:

```text
Project Dashboard
Script Editor
Scene Browser
Timeline Editor
Subtitle Editor
Voice Editor
Render Panel
```

## Phase 5 — AI Intelligence

```text
Character Recognition
VLM Scene Understanding
Visual Embeddings
Semantic Matching V2
Script ↔ Footage Feedback
Automatic Pacing
Hook Analysis
Emotion Analysis
```

## Phase 6 — Multi-output

Một project xuất:

```text
16:9
9:16
1:1
teaser
shorts
long recap
```

## Phase 7 — Production

```text
authentication
permissions
quota
billing
multi-user
GPU workers
distributed jobs
monitoring
autoscaling
```

---

# 42. Những việc nên làm đầu tiên sau khi Codex đọc tài liệu này

## Step 1

Không sửa code.

Đọc:

```text
pyproject.toml
src/movie_narrator/
src/movie_narrator/cloud/
src/movie_narrator/workflow/
src/movie_narrator/providers/
src/movie_narrator/tts/
src/movie_narrator/vision/
Dockerfile
docker-compose.yml
.github/workflows/
```

## Step 2

Chạy full test suite.

Tạo baseline report.

## Step 3

Tạo tài liệu architecture:

```text
docs/architecture-v1.md
docs/architecture-v2.md
docs/migration-plan.md
```

## Step 4

Tạo V2 skeleton nhưng không sửa V1:

```text
apps/
services/
packages/
infra/
```

## Step 5

Viết contracts:

```text
VideoEngine
Asset
Project
Job
Artifact
Timeline
PipelineNode
Provider
```

## Step 6

Viết LegacyMovieNarratorAdapter.

## Step 7

Tạo API mới gọi V1 qua adapter.

## Step 8

Sau khi hệ thống mới chạy end-to-end mới bắt đầu thay từng module.

---

# 43. Nguyên tắc cho Codex khi phát triển

1. Không rewrite toàn bộ engine trong một PR.
2. Không phá behavior cũ nếu chưa có replacement.
3. Mỗi V2 module phải có interface rõ.
4. Mỗi module mới phải có test.
5. Không đưa logic frontend/business vào engine.
6. Không hard-code model provider.
7. Không hard-code filesystem path.
8. Artifact phải có ID.
9. Pipeline phải resume được.
10. Pipeline phải partial-run được.
11. Timeline là nguồn sự thật của renderer.
12. AI tạo proposal; editor có thể override.
13. Storage phải abstraction.
14. Worker phải stateless càng nhiều càng tốt.
15. Media không tin cậy phải sandbox.
16. Secrets không được truyền vào render worker nếu không cần.
17. Không dùng shell=True.
18. Không auto-load plugin không tin cậy.
19. Không đổi namespace legacy hàng loạt.
20. Luôn giữ upstream remote để tham khảo.

---

# 44. Mục tiêu cuối cùng

Không phải:

```text
Movie → recap.mp4
```

Mà là:

```text
                     SOURCE VIDEO
                          │
                          ▼
                  Media Intelligence
                          │
             ┌────────────┼────────────┐
             ▼            ▼            ▼
         Characters     Scenes       Dialogue
             │            │            │
             └────────────┼────────────┘
                          ▼
                       AI Brain
                          │
             ┌────────────┼────────────┐
             ▼            ▼            ▼
          Script       Narration     Matching
             │            │            │
             └────────────┼────────────┘
                          ▼
                       Timeline
                          │
                   Human Editing
                          │
                          ▼
                       Renderer
                          │
               ┌──────────┼───────────┐
               ▼          ▼           ▼
            YouTube     Shorts      TikTok
```

Tên khái niệm phù hợp:

```text
AI Video Production Engine
```

Movie recap chỉ là một workflow trong hệ thống.

---

# 45. Tóm tắt quyết định kiến trúc

Ưu tiên:

```text
Project
Asset
Job
Pipeline Node
Checkpoint
Artifact
Timeline
Provider
Workflow
```

Không ưu tiên ngay:

```text
Kubernetes
microservices quá nhỏ
billing
multi-tenant
autoscaling
rewrite tất cả
```

Kiến trúc bắt đầu:

```text
Next.js
   ↓
FastAPI
   ↓
PostgreSQL
Redis
MinIO
   ↓
VideoEngine interface
   ↓
Legacy Movie Narrator Adapter
   ↓
Movie Narrator V1
```

Rồi dần chuyển sang:

```text
Next.js
   ↓
FastAPI
   ↓
V2 Pipeline
   ↓
AI / ML / Render Workers
   ↓
Timeline Renderer
```

---

# 46. Checklist khởi động

- [ ] Tạo repository riêng.
- [ ] Thêm upstream remote.
- [ ] Tag baseline upstream.
- [ ] Pin Python 3.13.
- [ ] Dùng uv.
- [ ] Cài FFmpeg.
- [ ] Chạy toàn bộ tests.
- [ ] Chạy một sample project end-to-end.
- [ ] Freeze baseline branch.
- [ ] Tạo monorepo skeleton.
- [ ] Tạo architecture documents.
- [ ] Tạo domain model.
- [ ] Tạo VideoEngine interface.
- [ ] Tạo LegacyMovieNarratorAdapter.
- [ ] Tạo Product API.
- [ ] Tạo Project / Asset / Job database.
- [ ] Tạo object storage abstraction.
- [ ] Tạo progress events.
- [ ] Tạo checkpoint/resume.
- [ ] Tạo partial pipeline.
- [ ] Tạo Timeline model.
- [ ] Tạo Script Editor.
- [ ] Tạo Scene Editor.
- [ ] Tạo Subtitle Editor.
- [ ] Tạo Voice Editor.
- [ ] Tạo VLM abstraction.
- [ ] Tạo Character Engine.
- [ ] Tạo Matching V2.
- [ ] Tạo Multi-output renderer.
- [ ] Hardening security.
- [ ] Sau cùng mới production scaling.

---

# 47. Ghi chú cho AI coding agent

Khi bắt đầu phát triển, hãy coi repo Movie Narrator hiện tại là:

```text
LEGACY ENGINE / REFERENCE IMPLEMENTATION
```

Không coi nó là architecture cuối cùng.

Mọi feature mới nên được thiết kế theo hướng:

```text
independent
testable
replaceable
provider-agnostic
storage-agnostic
UI-agnostic
resume-able
observable
```

Mục tiêu quan trọng nhất là đảm bảo:

```text
AI generation
+
human editing
+
repeatable rendering
+
multi-output
```

cùng tồn tại trong một project model thống nhất.

---

# 48. Nguồn tham khảo chính

Repo upstream:

https://github.com/zcbacxc/movie-narrator.git

Các file cần đọc lại khi bắt đầu code:

```text
README.md
pyproject.toml
Dockerfile
docker-compose.yml
SECURITY.md
CHANGELOG.md
docs/
src/movie_narrator/cloud/
src/movie_narrator/workflow/
src/movie_narrator/providers/
src/movie_narrator/tts/
src/movie_narrator/vision/
src/movie_narrator/utils/
.github/workflows/
```

---

# 49. Kết luận

Định hướng phù hợp nhất:

```text
Không fork để vá dần.
Không rewrite toàn bộ ngay.
Không nhét product logic vào core.

Giữ Movie Narrator làm Engine V1.
Xây architecture V2 bao quanh.
Dùng adapter để duy trì tính tương thích.
Thay từng module theo thời gian.
Lấy Timeline + Project + Pipeline Node + Artifact làm nền móng.
```

Nếu thực hiện đúng kiến trúc này, hệ thống có thể phát triển từ một movie recap tool thành một nền tảng AI video production có khả năng chỉnh sửa, tự động hóa, batch processing, multi-output và distributed rendering lâu dài.
