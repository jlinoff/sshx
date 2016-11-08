# Simple makefile to build sshx.
# Just type make.
sshx: preflight main.go getpassword.go options.go
	GOPATH=$$(pwd) go build -o $@ main.go getpassword.go options.go

preflight:
	GOPATH=$$(pwd) go get golang.org/x/crypto/ssh

help: sshx
	./sshx -h

test: sshx
	@cd test; make

