#include "block_store.hpp"
#include "tiny_storage.h"

#include <cstdlib>
#include <cstring>
#include <new>
#include <stdexcept>
#include <vector>

struct TinyStorage {
    tiny::BlockStore store;
};

TinyStorage* tiny_storage_open(const char* root_path) {
    if (root_path == nullptr) {
        return nullptr;
    }

    try {
        return new TinyStorage{tiny::BlockStore(root_path)};
    } catch (...) {
        return nullptr;
    }
}

void tiny_storage_close(TinyStorage* store) {
    delete store;
}

TinyStorageStatus tiny_storage_put(
    TinyStorage* store,
    const char* id,
    const uint8_t* data,
    size_t len) {
    if (store == nullptr || id == nullptr || (len > 0 && data == nullptr)) {
        return TINY_STORAGE_ERR;
    }

    try {
        store->store.put(id, std::vector<std::uint8_t>(data, data + len));
        return TINY_STORAGE_OK;
    } catch (...) {
        return TINY_STORAGE_ERR;
    }
}

TinyStorageStatus tiny_storage_get(
    TinyStorage* store,
    const char* id,
    uint8_t** out_data,
    size_t* out_len) {
    if (store == nullptr || id == nullptr || out_data == nullptr || out_len == nullptr) {
        return TINY_STORAGE_ERR;
    }

    *out_data = nullptr;
    *out_len = 0;

    try {
        const auto bytes = store->store.get(id);
        if (bytes.empty()) {
            *out_data = static_cast<uint8_t*>(std::malloc(1));
            if (*out_data == nullptr) {
                return TINY_STORAGE_ERR;
            }
            *out_len = 0;
            return TINY_STORAGE_OK;
        }

        *out_data = static_cast<uint8_t*>(std::malloc(bytes.size()));
        if (*out_data == nullptr) {
            return TINY_STORAGE_ERR;
        }

        std::memcpy(*out_data, bytes.data(), bytes.size());
        *out_len = bytes.size();
        return TINY_STORAGE_OK;
    } catch (const std::runtime_error&) {
        return TINY_STORAGE_NOT_FOUND;
    } catch (...) {
        return TINY_STORAGE_ERR;
    }
}

TinyStorageStatus tiny_storage_delete(TinyStorage* store, const char* id) {
    if (store == nullptr || id == nullptr) {
        return TINY_STORAGE_ERR;
    }

    try {
        store->store.remove(id);
        return TINY_STORAGE_OK;
    } catch (const std::runtime_error&) {
        return TINY_STORAGE_NOT_FOUND;
    } catch (...) {
        return TINY_STORAGE_ERR;
    }
}

void tiny_storage_free_buffer(uint8_t* data) {
    std::free(data);
}
