# How we deployed an Ubuntu app on an Arch Linux machine — from a Windows laptop

This is a real walkthrough of what we actually did. No edits, no cleanup. Just what happened.

## The setup

- **Dell laptop** running Arch Linux, sitting on the desk, connected to the home WiFi at `192.168.1.34`
- **HP laptop** running Windows, sitting on the couch

The goal: make the Dell run a Python app inside an Ubuntu environment, controlled entirely from the Windows laptop. No Docker installed anywhere.

## Step 1: Get the code on the Dell

On the Dell, clone the repo and build everything:

```bash
git clone https://github.com/OSS-Initiatives-IIIT-Sonepat/tiny-aws.git
cd tiny-aws
cd data-plane/compute/ec2-agent && cargo build && cd ../../..
cd data-plane/storage/object-store && cargo build && cd ../../..
```

This takes a few minutes the first time (Rust compilation).

## Step 2: Configure for network access

By default everything binds to `127.0.0.1` (localhost only). We need the Dell to accept connections from the Windows laptop.

Copy the example env file and edit it:

```bash
cp .env.example .env.local
```

Change these lines in `.env.local`:

```
REGISTRY_URL=http://192.168.1.34:9000
SCHEDULER_URL=http://192.168.1.34:9001
OBJECT_STORE_URL=http://192.168.1.34:7001
OBJECT_STORE_ADDR=0.0.0.0:7001
AGENT_ADVERTISE_ADDR=192.168.1.34
```

The key thing: `0.0.0.0` means "listen on all network interfaces", not just localhost.

## Step 3: Start the stack on the Dell

We needed three terminals on the Dell. This was the annoying part.

**Terminal 1** — the main stack (registry, scheduler, controller, load balancer):

```bash
cd ~/Desktop/aws/tiny-aws
set -a; source .env.local; set +a
./scripts/run-local.sh
```

Wait ~10 seconds. You'll see a bunch of "listening on" messages.

**Terminal 2** — the object store (had to start separately because it kept crashing before the registry was up):

```bash
cd ~/Desktop/aws/tiny-aws
set -a; source .env.local; set +a
cd data-plane/storage/object-store
cargo run
```

Should print: `object-store listening on 0.0.0.0:7001`

**Terminal 3** — the compute agent (needs sudo for Linux namespace stuff):

```bash
cd ~/Desktop/aws/tiny-aws
set -a; source .env.local; set +a
pkill -f ec2-agent    # kill the one run-local.sh started
cd data-plane/compute/ec2-agent
sudo -E cargo run      # -E keeps the env vars
```

Should print: `ec2-agent listening on 0.0.0.0:8080`

### The thing about `set -a; source .env.local; set +a`

We kept running into this. `source .env.local` loads the variables into your shell, but doesn't export them to child processes (like `cargo run`). The `set -a` makes bash auto-export everything. We forgot this twice and spent time debugging "why is it connecting to 127.0.0.1".

## Step 4: Verify from the Windows laptop

On the Windows laptop (PowerShell):

```powershell
curl.exe -s http://192.168.1.34:9000/health
# {"service":"registry","status":"healthy"}

curl.exe -s http://192.168.1.34:9001/health
# {"service":"scheduler","status":"healthy"}

curl.exe -s http://192.168.1.34:7001/health
# {"status":"healthy","service":"object-store"}
```

All three responding. We're in.

## Step 5: Write the app

On the Windows laptop, we created a folder with one file:

```
test-ubuntu/
└── tinyaws.build
```

Contents of `tinyaws.build`:

```
base: jammy
packages: python3
start: python3 -c "print('hello from ubuntu on arch')"
```

That's it. Three lines. "Give me Ubuntu 22.04 (jammy), install Python 3, run this command."

### Why `jammy` and not `ubuntu`

We actually tried `base: ubuntu` first and it failed. `debootstrap` needs the codename (`jammy`, `noble`, `bookworm`), not the distro name. Ubuntu 22.04 = jammy. We learned this the hard way.

## Step 6: Deploy

On the Windows laptop:

```powershell
$env:REGISTRY_URL = "http://192.168.1.34:9000"
$env:SCHEDULER_URL = "http://192.168.1.34:9001"
$env:OBJECT_STORE_URL = "http://192.168.1.34:7001"

cd E:\tiny-aws\control-plane\cli
go run . deploy "E:\tiny-aws\test-ubuntu" --wait
```

### What we saw

First run took about 10 minutes. The Dell's agent terminal was printing `debootstrap` output — downloading and unpacking Ubuntu packages one by one. It was downloading all of Ubuntu's base system from the internet and building a root filesystem from scratch.

```
deploy job job-4 started
job_id=job-4 node_id=arch-dell status=done command=""
exit_code=0
stdout:
hello from ubuntu on arch
```

**`hello from ubuntu on arch`**

A Python process running inside an Ubuntu filesystem on an Arch Linux machine, deployed from a Windows laptop across the room.

## What actually happened under the hood

1. The CLI on Windows zipped the `test-ubuntu/` folder
2. Uploaded the zip to the Dell's object store over HTTP
3. Submitted a job to the Dell's scheduler
4. The Dell's agent picked up the job, downloaded and extracted the zip
5. Agent found `tinyaws.build`, parsed it
6. Ran `debootstrap --variant=minbase jammy /var/lib/tinyaws/images/1e6d4a2743a9e7ca/` — built a full Ubuntu root filesystem
7. Ran `chroot <rootfs> apt install python3` inside it
8. Created an overlayfs mount (Ubuntu rootfs = read-only base, job scratch = writable layer)
9. Ran `unshare --pid --mount --ipc --fork --mount-proc` for namespace isolation
10. Ran `pivot_root` so the process sees the Ubuntu rootfs as `/` — Arch's filesystem is invisible
11. Executed `python3 -c "print('hello from ubuntu on arch')"` inside the isolated Ubuntu box
12. Captured stdout, reported back to the scheduler
13. Windows CLI polled the scheduler, got the result, printed it

No Docker. No VMs. No Kubernetes. Just `debootstrap`, `overlayfs`, `pivot_root`, `unshare`, and ~250 lines of Rust.

## Second deploy was instant

The rootfs is cached by content hash. Same `tinyaws.build` = same hash = skip the build. Second deploy took about 2 seconds instead of 10 minutes.

## The bugs we hit along the way

1. **`export` doesn't work in PowerShell** — had to use `$env:VAR = "value"` instead
2. **Object store bound to localhost** — needed `OBJECT_STORE_ADDR=0.0.0.0:7001`
3. **`source .env.local` doesn't export** — needed `set -a` before sourcing
4. **`base: ubuntu` failed** — debootstrap needs codenames like `jammy`, not `ubuntu`
5. **Quote nesting in shell script** — the sandbox's pivot_root script broke on single quotes inside single quotes
6. **Port conflict** — run-local.sh starts its own agent, had to kill it before starting the new one with sudo
7. **Rust 2024 edition** — stricter about `ref` in pattern matching, had to fix one line

Every single one of these is the kind of thing you only learn by actually doing it. The code was right. The real world just has more edges.

## What we ended up with

An Arch Linux laptop that can spin up Ubuntu (or Debian) environments on demand, controlled remotely from any machine on the network. Each app describes what Linux it needs in 3-4 lines. The environment is built from `apt`, cached, and run in full isolation.

```
tinyaws.build:           What the Dell does:
base: jammy              debootstrap → full Ubuntu rootfs
packages: python3        apt install python3
start: python3 app.py    overlayfs + pivot_root + exec
```

No Docker. No prebuilt images. No registry. Just apt and Linux.
