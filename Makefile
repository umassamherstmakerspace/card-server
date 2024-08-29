server:
	docker buildx build --platform linux/amd64,linux/arm64 -t mkrcx/card-server -f src/Dockerfile . --push