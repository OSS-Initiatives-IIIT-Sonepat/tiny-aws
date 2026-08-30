# Hello lambda demo
# Zips handler.py, creates a lambda function, and invokes it.
# Run from repo root: .\examples\lambda\deploy.ps1

$ErrorActionPreference = "Stop"

$here = "$PSScriptRoot"
$zip  = "$env:TEMP\hello-lambda.zip"

# zip the handler
Compress-Archive -Path "$here\handler.py" -DestinationPath $zip -Force
Write-Host "zipped handler.py -> $zip"

# create function (uploads zip + registers metadata)
tinyaws lambda create hello-lambda --runtime python3 --handler handler.handler --file $zip
Write-Host "function created"

# invoke
$result = tinyaws lambda invoke hello-lambda --event '{"name":"tiny-aws"}'
Write-Host "result: $result"
