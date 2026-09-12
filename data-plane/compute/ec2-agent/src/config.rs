pub fn registry_url() -> String {
    std::env::var("REGISTRY_URL").unwrap_or_else(|_| "http://127.0.0.1:9000".into())
}

pub fn scheduler_url() -> String {
    std::env::var("SCHEDULER_URL").unwrap_or_else(|_| "http://127.0.0.1:9001".into())
}

pub fn object_store_url() -> String {
    std::env::var("OBJECT_STORE_URL").unwrap_or_else(|_| "http://127.0.0.1:7001".into())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Mutex;

    // Env vars are process-global; serialize tests that mutate them.
    static ENV_LOCK: Mutex<()> = Mutex::new(());

    #[test]
    fn registry_url_default() {
        let _g = ENV_LOCK.lock().unwrap();
        unsafe { std::env::remove_var("REGISTRY_URL") };
        assert_eq!(registry_url(), "http://127.0.0.1:9000");
    }

    #[test]
    fn registry_url_from_env() {
        let _g = ENV_LOCK.lock().unwrap();
        unsafe { std::env::set_var("REGISTRY_URL", "http://registry:9000") };
        assert_eq!(registry_url(), "http://registry:9000");
        unsafe { std::env::remove_var("REGISTRY_URL") };
    }

    #[test]
    fn scheduler_url_default() {
        let _g = ENV_LOCK.lock().unwrap();
        unsafe { std::env::remove_var("SCHEDULER_URL") };
        assert_eq!(scheduler_url(), "http://127.0.0.1:9001");
    }

    #[test]
    fn scheduler_url_from_env() {
        let _g = ENV_LOCK.lock().unwrap();
        unsafe { std::env::set_var("SCHEDULER_URL", "http://sched:9001") };
        assert_eq!(scheduler_url(), "http://sched:9001");
        unsafe { std::env::remove_var("SCHEDULER_URL") };
    }

    #[test]
    fn object_store_url_default() {
        let _g = ENV_LOCK.lock().unwrap();
        unsafe { std::env::remove_var("OBJECT_STORE_URL") };
        assert_eq!(object_store_url(), "http://127.0.0.1:7001");
    }

    #[test]
    fn object_store_url_from_env() {
        let _g = ENV_LOCK.lock().unwrap();
        unsafe { std::env::set_var("OBJECT_STORE_URL", "http://store:7001") };
        assert_eq!(object_store_url(), "http://store:7001");
        unsafe { std::env::remove_var("OBJECT_STORE_URL") };
    }
}
