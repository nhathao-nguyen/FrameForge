"""Native scene, reference-style and profile-specific video analysis."""

from nh_media.gate_g import (
    ReframePlan,
    ReferenceStyleAnalysis,
    Scene,
    SceneAnalysis,
    SceneFeatures,
    analyze_reference_style,
    analyze_scenes,
    auto_reframe,
    detect_scenes,
    extract_scene_features,
    filter_scenes,
)

__all__ = ["ReframePlan", "ReferenceStyleAnalysis", "Scene", "SceneAnalysis", "SceneFeatures", "analyze_reference_style", "analyze_scenes", "auto_reframe", "detect_scenes", "extract_scene_features", "filter_scenes"]
