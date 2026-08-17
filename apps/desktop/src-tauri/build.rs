fn main() {
    // Gate B has no product artwork yet. Generate a deterministic 1x1 placeholder so the
    // Windows resource step remains compilable; branding belongs to a later client task.
    let icon_dir = std::path::Path::new("icons");
    std::fs::create_dir_all(icon_dir).expect("create Tauri icon directory");
    let icon_path = icon_dir.join("icon.ico");
    if !icon_path.exists() {
        let icon: &[u8] = &[
            0, 0, 1, 0, 1, 0, 1, 1, 0, 0, 1, 0, 32, 0, 48, 0, 0, 0, 22, 0, 0, 0, 40, 0, 0, 0, 1, 0,
            0, 0, 2, 0, 0, 0, 1, 0, 32, 0, 0, 0, 0, 0, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
            0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
        ];
        std::fs::write(&icon_path, icon).expect("write Tauri placeholder icon");
    }
    tauri_build::build();
}
