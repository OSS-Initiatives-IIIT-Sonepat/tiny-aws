pub fn registry_url() -> String {
    std::env::var("REGISTRY_URL").unwrap_or_else(|_| "http://127.0.0.1:9000".into())
}

pub fn scheduler_url() -> String {
    std::env::var("SCHEDULER_URL").unwrap_or_else(|_| "http://127.0.0.1:9001".into())
}

pub fn object_store_url() -> String {
    std::env::var("OBJECT_STORE_URL").unwrap_or_else(|_| "http://127.0.0.1:7001".into())
}