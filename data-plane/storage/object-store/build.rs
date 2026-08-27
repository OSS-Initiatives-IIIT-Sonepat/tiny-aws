use std::env;
use std::fs;
use std::path::PathBuf;

fn main() {
    println!("cargo:rerun-if-changed=../engine/include/tiny_storage.h");
    println!("cargo:rerun-if-changed=../engine/src");
    println!("cargo:rerun-if-changed=../engine/CMakeLists.txt");

    let profile = env::var("PROFILE").unwrap_or_else(|_| "debug".into());
    let cmake_profile = cmake_profile_name(&profile);

    let dst = cmake::Config::new("../engine")
        .profile(cmake_profile)
        .build();

    let lib_dir = dst.join("lib");
    let build_lib_dir = env::var("OUT_DIR")
        .map(|out| PathBuf::from(out).join("build").join(cmake_profile).join("lib"))
        .ok()
        .filter(|path| path.join(import_library_name()).exists());

    if lib_dir.join(import_library_name()).exists() {
        println!("cargo:rustc-link-search=native={}", lib_dir.display());
    } else if let Some(path) = build_lib_dir {
        println!("cargo:rustc-link-search=native={}", path.display());
    } else {
        panic!("could not locate tiny_storage import library");
    }

    println!("cargo:rustc-link-lib=dylib=tiny_storage");

    copy_shared_library(&dst, &profile);
}

fn cmake_profile_name(profile: &str) -> &str {
    if cfg!(target_env = "msvc") {
        match profile {
            "release" | "bench" => "Release",
            _ => "Debug",
        }
    } else {
        profile
    }
}

fn copy_shared_library(dst: &PathBuf, profile: &str) {
    let manifest_dir = PathBuf::from(env::var("CARGO_MANIFEST_DIR").unwrap());
    let target_dir = manifest_dir.join("target").join(profile);

    let candidates = [
        dst.join("bin").join(library_name()),
        dst.join("lib").join(library_name()),
    ];

    for candidate in candidates {
        if candidate.exists() {
            fs::create_dir_all(&target_dir).expect("failed to create target dir");
            fs::copy(&candidate, target_dir.join(candidate.file_name().unwrap()))
                .expect("failed to copy storage engine library");
            break;
        }
    }
}

fn import_library_name() -> &'static str {
    if cfg!(target_os = "windows") {
        "tiny_storage.lib"
    } else if cfg!(target_os = "macos") {
        "libtiny_storage.dylib"
    } else {
        "libtiny_storage.so"
    }
}

fn library_name() -> &'static str {
    if cfg!(target_os = "windows") {
        "tiny_storage.dll"
    } else if cfg!(target_os = "macos") {
        "libtiny_storage.dylib"
    } else {
        "libtiny_storage.so"
    }
}
