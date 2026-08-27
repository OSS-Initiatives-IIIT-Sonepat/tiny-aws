#pragma once

#include <cstdint>
#include <filesystem>
#include <string>
#include <vector>

namespace tiny {

class BlockStore {
public:
    explicit BlockStore(std::filesystem::path root);

    void put(const std::string& id, const std::vector<std::uint8_t>& data);
    std::vector<std::uint8_t> get(const std::string& id);
    void remove(const std::string& id);

private:
    std::filesystem::path root_;
    std::filesystem::path path_for(const std::string& id) const;
};

} // namespace tiny
