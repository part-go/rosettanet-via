compile:
	echo "Compiling for every OS and Platform"
	GOOS=freebsd GOARCH=386 go build -o bin/via-freebsd-386 cmd/via_server.go
	GOOS=linux GOARCH=386 go build -o bin/via-linux-386 cmd/via_server.go
	GOOS=windows GOARCH=386 go build -o bin/via-windows-386.exe cmd/via_server.go