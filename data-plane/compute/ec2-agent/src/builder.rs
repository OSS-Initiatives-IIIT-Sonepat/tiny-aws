// Builder: reads tinyaws.build from a deploy directory, builds a rootfs with the
// requested packages, and caches the result by content hash.
//
// tinyaws.build format (one key: value per line):
//   base: ubuntu            — debootstrap suite (default: uses existing base rootfs)
//   packages: python3 pip   — space-separated apt packages
//   run: pip install flask  — shell commands to run during build
//   start: python3 app.py   — what to exec at runtime (overrides start.sh)
//
// The built rootfs is cached at /var/lib/tinyaws/images/<sha256-of-build-file>/
// so identical build files skip the rebuild entirely.

use std::collections::hash_map::DefaultHasher;
use std::hash::{Hash, Hasher};
use std::path::{Path, PathBuf};
use std::process::Command;

/// Parsed tinyaws.build file.
#[derive(Debug, Clone)]
pub struct BuildSpec {
    pub base: String,
    pub packages: Vec<String>,
    pub run: Vec<String>,
    pub start: String,
}

impl BuildSpec {
    /// Reads and parses a tinyaws.build file. Returns None if file doesn't exist.
    pub fn from_dir(dir: &Path) -> Option<Self> {
        let path = dir.join("tinyaws.build");
        let content = std::fs::read_to_string(&path).ok()?;
        Some(Self::parse(&content))
    }

    fn parse(content: &str) -> Self {
        let mut base = String::new();
        let mut packages = Vec::new();
        let mut run = Vec::new();
        let mut start = String::new();

        for line in content.lines() {
            let line = line.trim();
            if line.is_empty() || line.starts_with('#') {
                continue;
            }
            if let Some((key, val)) = line.split_once(':') {
                let key = key.trim();
                let val = val.trim();
                match key {
                    "base" => base = val.to_string(),
                    "packages" => {
                        packages = val.split_whitespace()
                            .map(|s| s.to_string())
                            .collect();
                    }
                    "run" => run.push(val.to_string()),
                    "start" => start = val.to_string(),
                    _ => {} // ignore unknown keys
                }
            }
        }

        Self { base, packages, run, start }
    }

    /// Returns a stable hash of the build spec for caching.
    fn content_hash(&self) -> String {
        let mut hasher = DefaultHasher::new();
        self.base.hash(&mut hasher);
        self.packages.hash(&mut hasher);
        self.run.hash(&mut hasher);
        format!("{:016x}", hasher.finish())
    }
}

/// Where built images are cached.
fn images_dir() -> PathBuf {
    let dir = std::env::var("TINYAWS_IMAGES_DIR")
        .unwrap_or_else(|_| "/var/lib/tinyaws/images".into());
    PathBuf::from(dir)
}

/// Builds a rootfs from a BuildSpec and returns the path to the cached image.
/// If a cached image with the same hash exists, returns it immediately.
pub fn build_image(spec: &BuildSpec) -> Result<PathBuf, String> {
    let hash = spec.content_hash();
    let image_path = images_dir().join(&hash);

    // cache hit — return immediately
    if image_path.join("usr").exists() {
        println!("builder: using cached image {}", hash);
        return Ok(image_path);
    }

    println!("builder: building image {} ...", hash);

    // determine base rootfs
    let base = if spec.base.is_empty() {
        crate::instances::rootfs_base()
    } else {
        // check if it's a path or a debootstrap suite name
        let as_path = PathBuf::from(&spec.base);
        if as_path.exists() {
            as_path
        } else {
            // treat as debootstrap suite — build fresh
            return build_from_debootstrap(spec, &image_path);
        }
    };

    if !base.exists() {
        return Err(format!(
            "base rootfs not found at {}. Run scripts/bootstrap-rootfs.sh first.",
            base.display()
        ));
    }

    // copy base rootfs to image dir
    std::fs::create_dir_all(images_dir())
        .map_err(|e| format!("mkdir images: {}", e))?;

    run_cmd("cp", &["-a", &base.to_string_lossy(), &image_path.to_string_lossy()])?;

    // install packages
    if !spec.packages.is_empty() {
        let pkg_list = spec.packages.join(" ");
        println!("builder: installing packages: {}", pkg_list);
        let cmd = format!("apt-get update -qq && apt-get install -y -qq {}", pkg_list);
        run_in_chroot(&image_path, &cmd)?;
    }

    // run build commands
    for cmd in &spec.run {
        println!("builder: run: {}", cmd);
        run_in_chroot(&image_path, cmd)?;
    }

    println!("builder: image {} ready", hash);
    Ok(image_path)
}

/// Builds a rootfs from scratch using debootstrap for the given suite.
fn build_from_debootstrap(spec: &BuildSpec, image_path: &Path) -> Result<PathBuf, String> {
    std::fs::create_dir_all(images_dir())
        .map_err(|e| format!("mkdir images: {}", e))?;

    let suite = &spec.base;
    println!("builder: debootstrap {} ...", suite);
    run_cmd("debootstrap", &[
        "--variant=minbase",
        suite,
        &image_path.to_string_lossy(),
    ])?;

    // install packages
    if !spec.packages.is_empty() {
        let pkg_list = spec.packages.join(" ");
        println!("builder: installing packages: {}", pkg_list);
        let cmd = format!("apt-get update -qq && apt-get install -y -qq {}", pkg_list);
        run_in_chroot(image_path, &cmd)?;
    }

    // run build commands
    for cmd in &spec.run {
        println!("builder: run: {}", cmd);
        run_in_chroot(image_path, cmd)?;
    }

    println!("builder: image ready at {}", image_path.display());
    Ok(image_path.to_path_buf())
}

/// Runs a command inside a chroot.
fn run_in_chroot(rootfs: &Path, cmd: &str) -> Result<(), String> {
    let status = Command::new("chroot")
        .arg(rootfs)
        .args(["sh", "-c", cmd])
        .status()
        .map_err(|e| format!("chroot failed: {}", e))?;
    if !status.success() {
        return Err(format!("chroot command failed: {}", cmd));
    }
    Ok(())
}

fn run_cmd(prog: &str, args: &[&str]) -> Result<(), String> {
    let status = Command::new(prog)
        .args(args)
        .status()
        .map_err(|e| format!("{} failed: {}", prog, e))?;
    if !status.success() {
        return Err(format!("{} {:?} failed: {:?}", prog, args, status.code()));
    }
    Ok(())
}
