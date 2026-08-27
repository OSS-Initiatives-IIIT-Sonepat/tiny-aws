#pragma once

#include <stddef.h>
#include <stdint.h>

// cpp engine for storage
// contract between rust API and cpp engine
// cpp library that CRUDs over files on disk,
// exposed via C Api so Rust can call it
#ifdef __cplusplus
extern "C" {

#endif

// Opaque handle - Rust never sees inside the c++ object
typedef struct TinyStorage TinyStorage;

typedef enum {
    TINY_STORAGE_OK = 0,
    TINY_STORAGE_ERR = -1,
    TINY_STORAGE_NOT_FOUND = -2,
} TinyStorageStatus;

TinyStorage* tiny_storage_open(const char* root_path);
void         tiny_storage_close(TinyStorage* store);

// Write block bytes to disk
TinyStorageStatus tiny_storage_put(
    TinyStorage* store,
    const char* id,
    const uint8_t* data,
    size_t len
);

// Read block — caller must call tiny_storage_free_buffer when done
TinyStorageStatus tiny_storage_get(
    TinyStorage* store,
    const char* id,
    uint8_t** out_data,
    size_t* out_len
);

TinyStorageStatus tiny_storage_delete(TinyStorage* store, const char* id);

void tiny_storage_free_buffer(uint8_t* data);

#ifdef __cplusplus
}
#endif