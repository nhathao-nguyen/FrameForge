#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

#[tauri::command]
fn validate_server_endpoint(endpoint: String) -> Result<String, String> {
    let value = endpoint.trim().trim_end_matches('/');
    let authority = value
        .strip_prefix("https://")
        .or_else(|| value.strip_prefix("http://"))
        .unwrap_or("");
    if value.is_empty()
        || value.contains(char::is_whitespace)
        || (!value.starts_with("http://") && !value.starts_with("https://"))
        || authority.is_empty()
    {
        return Err("The server endpoint must be an explicit HTTP(S) URL.".into());
    }
    if value.contains("..") || value.contains('@') || value.contains('#') {
        return Err("The server endpoint contains an unsafe authority.".into());
    }
    Ok(value.to_owned())
}

#[tauri::command]
fn client_boundary() -> &'static str {
    "remote_product_api_only"
}

fn main() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![
            validate_server_endpoint,
            client_boundary
        ])
        .run(tauri::generate_context!())
        .expect("error while running NH-Media desktop");
}
