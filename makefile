all:
    go build -o build/interface ./src
cross:
    GOOS=linux GOARCH=armv7 go build -o build/interface ./src
