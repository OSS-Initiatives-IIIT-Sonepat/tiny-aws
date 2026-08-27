.PHONY: registry agent storage

registry:
	cd control-plane/registry && go run .

agent:
	cd data-plane/compute/ec2-agent && cargo run

storage:
	cd data-plane/storage/object-store && cargo run