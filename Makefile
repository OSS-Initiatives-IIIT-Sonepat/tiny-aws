.PHONY: registry agent storage scheduler smoke

registry:
	cd control-plane/registry && go run .

agent:
	cd data-plane/compute/ec2-agent && cargo run

storage:
	cd data-plane/storage/object-store && cargo run

scheduler:
	cd control-plane/scheduler && go run .

smoke:
	powershell -ExecutionPolicy Bypass -File tests/integration/smoke-test.ps1