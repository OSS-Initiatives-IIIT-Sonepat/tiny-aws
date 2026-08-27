use std::os::raw::{c_char, c_int};

#[repr(C)]
pub struct TinyStorage {
    _private: [u8; 0],
}

pub const TINY_STORAGE_OK: c_int = 0;
pub const TINY_STORAGE_ERR: c_int = -1;
pub const TINY_STORAGE_NOT_FOUND: c_int = -2;

unsafe extern "C" {
    pub fn tiny_storage_open(root_path: *const c_char) -> *mut TinyStorage;
    pub fn tiny_storage_close(store: *mut TinyStorage);

    pub fn tiny_storage_put(
        store: *mut TinyStorage,
        id: *const c_char,
        data: *const u8,
        len: usize,
    ) -> c_int;

    pub fn tiny_storage_get(
        store: *mut TinyStorage,
        id: *const c_char,
        out_data: *mut *mut u8,
        out_len: *mut usize,
    ) -> c_int;

    pub fn tiny_storage_delete(store: *mut TinyStorage, id: *const c_char) -> c_int;

    pub fn tiny_storage_free_buffer(data: *mut u8);
}
