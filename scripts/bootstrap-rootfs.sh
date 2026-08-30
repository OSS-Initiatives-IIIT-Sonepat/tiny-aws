#!/bin/bash
# Bootstrap the base rootfs image for tiny-aws instances.
# Run once on each machine that will host instances.
# Requires: debootstrap, systemd-nspawn (usually in systemd-container package)
#
# Usage: sudo ./scripts/bootstrap-rootfs.sh [debian|ubuntu] [bookworm|jammy]

set -e

DISTRO="${1:-debian}"
SUITE="${2:-bookworm}"
BASE_DIR="${TINYAWS_ROOTFS_BASE:-/var/lib/tinyaws/base}"
INSTANCES_DIR="${TINYAWS_INSTANCES_DIR:-/var/lib/tinyaws/instances}"

if [ "$(id -u)" -ne 0 ]; then
  echo "Run as root: sudo $0"
  exit 1
fi

command -v debootstrap > /dev/null || {
  echo "Installing debootstrap..."
  apt-get install -y debootstrap
}

command -v systemd-nspawn > /dev/null || {
  echo "Installing systemd-container..."
  apt-get install -y systemd-container
}

mkdir -p "$BASE_DIR" "$INSTANCES_DIR"

if [ -d "$BASE_DIR/usr" ]; then
  echo "Base rootfs already exists at $BASE_DIR — skipping debootstrap."
  echo "Delete $BASE_DIR and re-run to rebuild."
  exit 0
fi

echo "Creating base rootfs ($DISTRO $SUITE) at $BASE_DIR ..."
echo "This takes 2-5 minutes."

debootstrap \
  --include=systemd,dbus,curl,unzip,python3,nodejs \
  "$SUITE" \
  "$BASE_DIR"

# minimal config inside the base image
echo "tinyaws-instance" > "$BASE_DIR/etc/hostname"
echo -e "[Network]\nDHCP=yes" > "$BASE_DIR/etc/systemd/network/20-eth.network"

# disable password for root inside containers (agent uses exec, not login)
chroot "$BASE_DIR" passwd -d root

echo ""
echo "Base rootfs ready at $BASE_DIR"
echo ""
echo "Now start the ec2-agent and launch instances:"
echo "  tinyaws instance launch --type small"
echo ""
echo "Instance types:"
echo "  nano   — 25% CPU, 128MB RAM"
echo "  micro  — 50% CPU, 256MB RAM"
echo "  small  — 100% CPU, 512MB RAM  (default)"
echo "  medium — 200% CPU, 1GB RAM"
echo "  large  — 400% CPU, 2GB RAM"
