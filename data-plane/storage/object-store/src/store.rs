use crate::block::Block;
use crate::ffi::{
    self, TinyStorage, TINY_STORAGE_ERR, TINY_STORAGE_NOT_FOUND, TINY_STORAGE_OK,
};
use std::ffi::CString;
use std::path::Path;

pub struct BlockStore {
    handle: *mut TinyStorage,
}

impl BlockStore {
    pub fn new(root: impl AsRef<Path>) -> std::io::Result<Self> {
        let root = root.as_ref().to_string_lossy();
        let c_root = CString::new(root.as_ref())
            .map_err(|_| std::io::Error::new(std::io::ErrorKind::InvalidInput, "bad path"))?;

        let handle = unsafe { ffi::tiny_storage_open(c_root.as_ptr()) };
        if handle.is_null() {
            return Err(std::io::Error::other("failed to open storage engine"));
        }

        Ok(Self { handle })
    }

    pub fn write_block(&self, block: &Block) -> std::io::Result<()> {
        let c_id = CString::new(block.id.as_str())
            .map_err(|_| std::io::Error::new(std::io::ErrorKind::InvalidInput, "bad id"))?;

        let status = unsafe {
            ffi::tiny_storage_put(
                self.handle,
                c_id.as_ptr(),
                block.data.as_ptr(),
                block.data.len(),
            )
        };

        match status {
            TINY_STORAGE_OK => Ok(()),
            TINY_STORAGE_NOT_FOUND => Err(std::io::Error::new(
                std::io::ErrorKind::NotFound,
                "block not found",
            )),
            TINY_STORAGE_ERR => Err(std::io::Error::other("put failed")),
            _ => Err(std::io::Error::other("put failed")),
        }
    }

    pub fn read_block(&self, id: &str) -> std::io::Result<Vec<u8>> {
        let c_id = CString::new(id)
            .map_err(|_| std::io::Error::new(std::io::ErrorKind::InvalidInput, "bad id"))?;

        let mut out_data: *mut u8 = std::ptr::null_mut();
        let mut out_len: usize = 0;

        let status =
            unsafe { ffi::tiny_storage_get(self.handle, c_id.as_ptr(), &mut out_data, &mut out_len) };

        match status {
            TINY_STORAGE_OK => {
                let slice = unsafe { std::slice::from_raw_parts(out_data, out_len) };
                let data = slice.to_vec();
                unsafe { ffi::tiny_storage_free_buffer(out_data) };
                Ok(data)
            }
            TINY_STORAGE_NOT_FOUND => Err(std::io::Error::new(
                std::io::ErrorKind::NotFound,
                "block not found",
            )),
            TINY_STORAGE_ERR => Err(std::io::Error::other("get failed")),
            _ => Err(std::io::Error::other("get failed")),
        }
    }

    pub fn delete_block(&self, id: &str) -> std::io::Result<()> {
        let c_id = CString::new(id)
            .map_err(|_| std::io::Error::new(std::io::ErrorKind::InvalidInput, "bad id"))?;

        let status = unsafe { ffi::tiny_storage_delete(self.handle, c_id.as_ptr()) };

        match status {
            TINY_STORAGE_OK => Ok(()),
            TINY_STORAGE_NOT_FOUND => Err(std::io::Error::new(
                std::io::ErrorKind::NotFound,
                "block not found",
            )),
            TINY_STORAGE_ERR => Err(std::io::Error::other("delete failed")),
            _ => Err(std::io::Error::other("delete failed")),
        }
    }
}

impl Drop for BlockStore {
    fn drop(&mut self) {
        unsafe { ffi::tiny_storage_close(self.handle) };
    }
}

unsafe impl Send for BlockStore {}
unsafe impl Sync for BlockStore {}
