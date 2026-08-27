cmake_minimum_required(VERSION 3.20)
project(tiny_storage LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

add_library(tiny_storage SHARED
    src/block_store.cpp
    src/ffi.cpp
)

target_include_directories(tiny_storage PUBLIC include)
