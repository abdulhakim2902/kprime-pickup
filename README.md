## Pickup Manager

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