@echo off
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -o build/wallet-service ./services/wallet-service/cmd
