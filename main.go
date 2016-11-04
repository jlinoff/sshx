// Demonstration program to demonstrate some aspects of the SSH package.
/*
License: The MIT License (MIT)

Copyright (c) 2016 Joe Linoff

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject
to the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR
ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF
CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/
package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path"
	"runtime"
	"strings"

	"golang.org/x/crypto/ssh"
)

//var version = "0.1" // initial release
//var version = "0.2" // Fixed the public-key handling, added vinfo and vinfon
var version = "0.3" // Added -A to support custom settings for HostKeyAlgorithms

func main() {
	// This is a hard-coded test of SSH.
	opts := getopts()
	if len(os.Args) > 1 {
		config := sshClientConfig(opts)
		exec(opts.Command, opts.Host, config)
	}
}

// prompt
func prompt(p string, d string) (value string) {
	if d == "" {
		fmt.Print(p + ": ")
	} else {
		fmt.Printf("%v <%v>: ", p, d)
	}
	r := bufio.NewReader(os.Stdin)
	value, _ = r.ReadString('\n')
	value = strings.TrimSpace(value)
	if value == "" {
		value = d
	}
	return
}

// Create the ssh config structure for:
//    password
//    keyboard-interactive
//    publickey
func sshClientConfig(opts options) (config *ssh.ClientConfig) {
	// Get the user's password.
	password := ""
	if opts.SSHPassword || opts.SSHKeyboardInteractive {
		if len(opts.Password) == 0 {
			password = getPassword(fmt.Sprintf("%v's password: ", opts.Username))
		} else {
			password = opts.Password
		}
	}

	// This will not work if the following command fails.
	// $ ssh -o PreferredAuthentications=password localhost pwd
	// Permission denied (publickey,keyboard-interactive).
	config = &ssh.ClientConfig{
		User: opts.Username,
	}

	// Use a custom set of host key algorithms if the user specified it.
	if len(opts.HostKeyAlgorithms) > 0 {
		as := strings.Join(opts.HostKeyAlgorithms, ",")
		vinfo(opts, "updating host key algorithms: [ %v ]", as)
		config.HostKeyAlgorithms = opts.HostKeyAlgorithms
	}
	// auth: public-key
	// Get the public key, if it is available.
	if opts.SSHPublicKey {
		vinfo(opts, "auth: public-key")
		if userData, err1 := user.Lookup(opts.Username); err1 == nil {
			sshDir := path.Join(userData.HomeDir, ".ssh")
			if _, err2 := os.Stat(sshDir); err2 == nil {
				// The ~/.ssh directory exists look for id_ files that do
				// do not have the .pub extension. Add an auth entry for
				// each one.
				// Typically they will be things like id_rsa or id_ecdsa.
				files, _ := ioutil.ReadDir(sshDir)
				for _, f := range files {
					fn := f.Name()
					if strings.HasPrefix(fn, "id_") && strings.HasSuffix(fn, ".pub") == false {
						keyFile := path.Join(sshDir, fn)
						vinfo(opts, "   keyFile = %v", keyFile)
						if key, err3 := ioutil.ReadFile(keyFile); err3 == nil {
							if signer, err4 := ssh.ParsePrivateKey(key); err4 == nil {
								config.Auth = append(config.Auth, ssh.PublicKeys(signer))
							} else {
								vinfo(opts, "%v", err4)
							}
						} else {
							vinfo(opts, "%v", err3)
						}
					} else {
						vinfon(opts, 2, "ignoring %v", fn)
					}
				} // for loop
			} else {
				vinfo(opts, "%v", err1)
			}
		}
	}

	// auth: password
	if opts.SSHPassword {
		vinfo(opts, "auth: password")
		config.Auth = append(config.Auth, ssh.Password(password))
	}

	// auth: keyboard-interactive
	if opts.SSHKeyboardInteractive {
		vinfo(opts, "auth: keyboard-interactive")

		// This will be called if SSH keyboard-interactive is enabled and
		// password is disabled. Same as:
		//    ssh -o PreferredAuthentications=password,keyboard-interactive
		// See RFC-4252 for details of how the callbacks work.
		kbic := func(
			user,
			instruction string,
			questions []string,
			echos []bool) (answers []string, err error) {
			// Callback, will be called multiple times.
			if len(questions) == 0 {
				return []string{}, nil
			}
			if len(questions) == 1 {
				return []string{password}, nil
			}
			panic(fmt.Errorf("ERROR: unexpected authentication chain"))
		}

		config.Auth = append(config.Auth, ssh.KeyboardInteractive(kbic))
	}

	return
}

// Execute the command.
func exec(cmd string, addr string, config *ssh.ClientConfig) {
	// Create the connection.
	conn, err := ssh.Dial("tcp", addr, config)
	check(err)
	session, err := conn.NewSession()
	check(err)
	defer session.Close()

	// Collect the output from stdout and stderr.
	// The idea is to duplicate the shell IO redirection
	// comment 2>&1 where both streams are interleaved.
	stdoutPipe, err := session.StdoutPipe()
	check(err)
	stderrPipe, err := session.StderrPipe()
	check(err)
	outputReader := io.MultiReader(stdoutPipe, stderrPipe)
	outputScanner := bufio.NewScanner(outputReader)

	// Start the session.
	err = session.Start(cmd)
	check(err)

	// Capture the output asynchronously.
	outputLine := make(chan string)
	outputDone := make(chan bool)
	go func(scan *bufio.Scanner, line chan string, done chan bool) {
		defer close(line)
		defer close(done)
		for scan.Scan() {
			line <- scan.Text()
		}
		done <- true
	}(outputScanner, outputLine, outputDone)

	// Use a custom wait.
	outputBuf := ""
	running := true
	for running {
		select {
		case <-outputDone:
			running = false
		case line := <-outputLine:
			outputBuf += line + "\n"
		}
	}
	session.Close()

	// Output the data.
	fmt.Print(outputBuf)
}

func check(e error) {
	if e != nil {
		_, _, lineno, _ := runtime.Caller(1)
		log.Fatalf("ERROR:%v %v", lineno, e)
	}
}

// Print an info message in verbose mode.
func vinfo(opts options, f string, args ...interface{}) {
	if opts.Verbose > 0 {
		_, _, lineno, _ := runtime.Caller(1)
		f1 := fmt.Sprintf("INFO::%04v %v", lineno, f)
		log.Printf(f1, args...)
	}
}

// Print an info message for a specific level of verbosity.
func vinfon(opts options, level int, f string, args ...interface{}) {
	if opts.Verbose >= level {
		_, _, lineno, _ := runtime.Caller(1)
		f1 := fmt.Sprintf("INFO::%04v %v", lineno, f)
		log.Printf(f1, args...)
	}
}
