// implement the declared class:

#include "block_store.hpp"
#include <fstream>
#include <stdexcept>

namespace tiny {

BlockStore::BlockStore(std::filesystem::path root)
    : root_(std::move(root)) {
    std::filesystem::create_directories(root_);
}

std::filesystem::path BlockStore::path_for(const std::string& id) const {
    return root_ / id;
}

void BlockStore::put(const std::string& id, const std::vector<uint8_t>& data) {
    auto path = path_for(id);
    if (path.has_parent_path()) {
        std::filesystem::create_directories(path.parent_path());
    }
    std::ofstream out(path, std::ios::binary);
    if (!out) throw std::runtime_error("failed to write block");
    out.write(reinterpret_cast<const char*>(data.data()), data.size());
}

std::vector<uint8_t> BlockStore::get(const std::string& id) {
    auto path = path_for(id);
    if (!std::filesystem::exists(path)) {
        throw std::runtime_error("block not found");
    }
    std::ifstream in(path, std::ios::binary | std::ios::ate);
    if (!in) throw std::runtime_error("failed to read block");
    auto size = in.tellg();
    in.seekg(0);
    std::vector<uint8_t> buf(size);
    in.read(reinterpret_cast<char*>(buf.data()), size);
    return buf;
}

void BlockStore::remove(const std::string& id) {
    if (!std::filesystem::remove(path_for(id))) {
        throw std::runtime_error("block not found");
    }
}

} 