# Go media worker boundary

This boundary owns Go media orchestration and FFmpeg/ffprobe adapters. The bounded argv-only process
shell, probe/thumbnail node and Redis worker-controller transport are implemented; storage-aware
materialization and production executable wiring remain behind the next runtime acceptance gate.

Gate G adds `internal/render`: a deterministic compiler from one validated TimelineVersion plus an
exact RenderProfile to an allowlisted FFmpeg argv plan, followed by ffprobe-based deliverable QA.
The compiler never calls providers or rematches proposals. Profile-only renders retain one
profile-independent upstream fingerprint; subject-aware auto-reframe must arrive as a typed
profile-specific intermediate rather than silently becoming a center crop.

The same boundary exposes typed clip/short export selections. `ExportClips` runs the allowlisted
FFmpeg profile, inspects the output with ffprobe, and returns a checksum/provenance manifest only
after QA passes; arbitrary FFmpeg arguments and sandbox-escaping paths are rejected.
