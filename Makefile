build: *.go **/*.go
	env GOOS=linux GOARCH=arm GOARM=5 go build -o build/switchboard
