// declare the class: BlockStore

#pragma once
#include <string>
#include <vector>
#include <filesystem>

namespace tiny {

class BlockStore {
public:
    explicit BlockStore(std::filesystem::path root);

    void put(const std::string& id, const std::vector<uint8_t>& data);
    std::vector<uint8_t> get(const std::string& id);
    void remove(const std::string& id);

private:
    std::filesystem::path root_;
    std::filesystem::path path_for(const std::string& id) const;
};

}