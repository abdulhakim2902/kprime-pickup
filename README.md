## Pickup Manager

### Setup the application
To setup the application, please copy file `.env.example` and rename it to `.env`, modify .env file based on your local requirement.
Add GOPRIVATE to the go env for git.devucc.name to get private dependencies

```bash
GOPRIVATE=github.com/Undercurrent-Technologies/kprime-utilities/ go get github.com/Undercurrent-Technologies/kprime-utilities/@v1.0.3
```

### Start the application
To start running the application, simply run:

```bash
go run main.go
```

To start running the application with docker:
#### Run pickup manager with Mongo and Kafka
```bash
export ACCESS_TOKEN={GITHUB_PERSONAL_ACCESS_TOKEN}
```
```bash
docker-compose -f ./.maintain/docker/docker-compose.prerequisite.yml -f docker-compose.yml up
```
#### Run pickup manager only
To setup the application, please copy file `.env.example` and rename it to `.env`, modify .env file based on your local requirement.
```bash
export ACCESS_TOKEN={GITHUB_PERSONAL_ACCESS_TOKEN}
```
```bash
docker-compose -f ./.maintain/docker/docker-compose.yml up
```
#### Run prerequisite only (Mongo and Kafka)
```bash
docker-compose -f ./.maintain/docker/docker-compose.prerequisite.yml up
```

## Build image
Go to ./maintain/docker directory to build the image.
```bash
export ACCESS_USER={GITHUB_PERSONAL_ACCESS_USER}
export ACCESS_TOKEN={GITHUB_PERSONAL_ACCESS_TOKEN}
```
```bash
docker compose build pickup
```