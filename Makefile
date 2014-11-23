all: build

build:
	gxc -target="linux/amd64" build -o api

docker:
	docker build --tag="joshrendek/errata_api" .

run:
	docker run -p 8081:80 -i -t joshrendek/errata_api

tag:
	docker tag errata_api:latest joshrendek/errata_api:latest

push:
	docker push joshrendek/errata_api:latest

dockerhub: build docker tag push

local_deploy: build docker run
