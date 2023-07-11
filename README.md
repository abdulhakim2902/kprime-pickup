## Pickup Manager

### Setup the application
To setup the application, please copy file `.env.example` and rename it to `.env`, modify .env file based on your local requirement.
Add GOPRIVATE to the go env for git.devucc.name to get private dependencies

```
GOPRIVATE=github.com/Undercurrent-Technologies/kprime-utilities/ go get github.com/Undercurrent-Technologies/kprime-utilities/@v1.0.3
```

### Start the application
To start running the application, simply run:

```
go run main.go
```

To start running the application with docker:
#### Run pickup manager with Mongo and Kafka
```
export ACCESS_TOKEN={GITLAB_PERSONAL_ACCESS_TOKEN}
```
```
docker-compose -f docker-compose.prerequisite.yml -f docker-compose.yml up
```
#### Run pickup manager only
To setup the application, please copy file `.env.example` and rename it to `.env`, modify .env file based on your local requirement.
```
export ACCESS_TOKEN={GITLAB_PERSONAL_ACCESS_TOKEN}
```
```
docker-compose -f docker-compose.yml up
```
#### Run prerequisite only (Mongo and Kafka)
```
docker-compose -f docker-compose.prerequisite.yml up
```

## Build image
```
export ACCESS_TOKEN={GITLAB_PERSONAL_ACCESS_TOKEN}
```
```
docker compose build pickup
```