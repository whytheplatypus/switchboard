build: *.go **/*.go
	env GOOS=linux GOARCH=arm GOARM=5 go build -o build/switchboard

coverage.out: *.go **/*.go
	go test -coverprofile=coverage.out ./...

cover: coverage.out
	go tool cover -html=coverage.out
