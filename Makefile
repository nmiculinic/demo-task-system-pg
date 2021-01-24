up:
	docker-compose up -d

clean:
	docker-compose down --volumes

migrate:
	@docker run --rm --net=host postgres:13 echo 'SELECT VERSION();' | psql ${DATABASE_URL}
	@docker run --rm -e DATABASE_URL=${DATABASE_URL} -v $$(pwd)/migrations:/migrations:ro --net=host amacneil/dbmate:v1.11.0 --no-dump-schema --migrations-dir /migrations up
