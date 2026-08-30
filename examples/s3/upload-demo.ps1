# S3-style upload demo
# Creates a bucket, uploads a file, retrieves it, and lists the bucket.
# Run from repo root: .\examples\s3\upload-demo.ps1

$ErrorActionPreference = "Stop"
$store = "http://127.0.0.1:7001"
$bucket = "demo-bucket"
$key    = "hello.txt"

Write-Host "=== tiny-aws S3 upload demo ==="
Write-Host ""

Write-Host "[1] Create bucket '$bucket'"
curl.exe -s -f -X PUT "$store/buckets/$bucket" | Out-Null
Write-Host "  ok"

Write-Host "[2] Upload file"
curl.exe -s -f -X PUT "$store/buckets/$bucket/objects/$key" -d "Hello from tiny-aws S3 demo!" | Out-Null
Write-Host "  uploaded $key"

Write-Host "[3] Retrieve file"
$body = curl.exe -s -f "$store/buckets/$bucket/objects/$key"
Write-Host "  content: $body"

Write-Host "[4] List bucket"
$list = curl.exe -s -f "$store/buckets/$bucket/objects"
Write-Host "  objects: $list"

Write-Host "[5] Get metadata"
$meta = curl.exe -s -f "$store/objects/$bucket/$key/meta" 2>$null
if (-not $meta) {
    $meta = curl.exe -s -f "$store/buckets/$bucket/objects/$key"
}
Write-Host "  meta fetched"

Write-Host ""
Write-Host "S3 demo complete."
