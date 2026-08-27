use std::path::PathBuf;

pub fn registry_url() -> String {
    std::env::var("REGISTRY_URL").unwrap_or_else(|_| "http://127.0.0.1:9000".into())
}

pub fn listen_addr() -> String {
    std::env::var("OBJECT_STORE_ADDR").unwrap_or_else(|_| "127.0.0.1:7001".into())
}

pub fn storage_root() -> PathBuf {
    PathBuf::from(
        std::env::var("STORAGE_ROOT").unwrap_or_else(|_| "data".into()),
    )
}

pub fn metadata_db() -> PathBuf {
    PathBuf::from(
        std::env::var("METADATA_DB").unwrap_or_else(|_| "metadata.db".into()),
    )
}