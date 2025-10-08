@echo off
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -o build/orchestrator-service ./services/orchestrator-service/cmd
