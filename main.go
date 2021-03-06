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
	"time"

	"golang.org/x/crypto/ssh"
)

//var version = "0.1" // initial release
//var version = "0.2" // Fixed the public-key handling, added vinfo and vinfon
//var version = "0.3" // Added -A to support custom settings for HostKeyAlgorithms
//var version = "0.4" // Added support for a remote terminal
//var version = "0.5" // Add support for multiple hosts
//var version = "0.6" // Add support for max jobs
//var version = "0.7" // Add support for timeout
//var version = "0.8" // Add retries for the TCP dial operation
var version = "0.8.1" // Fix error recovery in goroutine

func main() {
	// This is a hard-coded test of SSH.
	opts := getopts()
	if len(opts.Hosts) == 0 {
		os.Exit(0)
	}

	// Setup a goroutine to timeout if the user requested it.
	if opts.TimeoutSecs > 0 {
		go func(s int) {
			time.Sleep(time.Duration(s) * time.Second)
			fatal("timed out after %v seconds", s)
			os.Exit(1)
		}(opts.TimeoutSecs)
	}

	// Check for the case of no-command, that implies a remote terminal for
	// a single host.
	if len(opts.Command) == 0 {
		if len(opts.Hosts) == 1 {
			loadSSHConfig(opts)
			execTerm(opts)
		} else {
			fatal("cannot spawn remote shells on multiple hosts")
		}
	} else {
		execCmdsInParallel(opts)
	}
}

// load SSH configuration data.
func loadSSHConfig(opts options) {
	for i, hi := range opts.Hosts {
		opts.Hosts[i].Config = sshClientConfig(hi, opts)
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
func sshClientConfig(hi hostinfo, opts options) (config *ssh.ClientConfig) {
	vinfo(opts, "configuring ssh for [%v] %v@%v", hi.ID, hi.Username, hi.Host)

	username := hi.Username
	password := hi.Password
	host := hi.Host

	// Get the user's password.
	if opts.SSHPassword || opts.SSHKeyboardInteractive {
		if len(opts.Password) == 0 {
			password = getPassword(fmt.Sprintf("%v@%v's password: ", username, host))
		} else {
			password = opts.Password
		}
	}

	config = &ssh.ClientConfig{
		User: username,
	}

	// Use a custom set of host key algorithms if the user specified it.
	if len(opts.HostKeyAlgorithms) > 0 {
		as := strings.Join(opts.HostKeyAlgorithms, ",")
		vinfo(opts, "   updating host key algorithms: [ %v ]", as)
		config.HostKeyAlgorithms = opts.HostKeyAlgorithms
	}

	// auth: public-key
	// Get the public key, if it is available.
	if opts.SSHPublicKey {
		vinfo(opts, "   auth: public-key")
		if userData, err1 := user.Lookup(username); err1 == nil {
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
						vinfo(opts, "      keyFile = %v", keyFile)
						if key, err3 := ioutil.ReadFile(keyFile); err3 == nil {
							if signer, err4 := ssh.ParsePrivateKey(key); err4 == nil {
								config.Auth = append(config.Auth, ssh.PublicKeys(signer))
							} else {
								vinfo(opts, "         %v", err4)
							}
						} else {
							vinfo(opts, "         %v", err3)
						}
					} else {
						vinfon(opts, 2, "      ignoring %v", fn)
					}
				} // for loop
			} else {
				vinfo(opts, "   %v", err1)
			}
		}
	}

	// auth: password
	if opts.SSHPassword {
		vinfo(opts, "   auth: password")
		config.Auth = append(config.Auth, ssh.Password(password))
	}

	// auth: keyboard-interactive
	if opts.SSHKeyboardInteractive {
		vinfo(opts, "   auth: keyboard-interactive")

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

// Execute the commands in parallel.
func execCmdsInParallel(opts options) {
	loadSSHConfig(opts)

	hiChan := make(chan hostinfo, opts.MaxParallelJobs)

	// lambda that acts at the channel sink
	sink := func(m int, c chan hostinfo) {
		for i := 0; i < m; i++ {
			hi := <-c
			if opts.JobHeader {
				fmt.Printf(`
# ================================================================
# Job  : %[1]v
# User : %[2]v
# Host : %[3]v
# Cmd  : %[4]v
# Size : %[5]v
# ================================================================
%[6]v
`, hi.ID, hi.Username, hi.Host, opts.Command, len(hi.Output), hi.Output)
			} else {
				fmt.Print(hi.Output)
			}
		}
	}

	// Spawn the commands in parallel.
	// Honor the max parallel jobs setting.
	for j, hi := range opts.Hosts {
		vinfon(opts, 2, "spawning job %v", j)
		go execCmd(hi, opts, hiChan) // source
		if opts.MaxParallelJobs < 2 {
			vinfon(opts, 2, "sinking 1 job")
			sink(1, hiChan)
		} else {
			if j > 0 && (j%opts.MaxParallelJobs) == 0 {
				vinfon(opts, 2, "sinking %v jobs", opts.MaxParallelJobs)
				sink(opts.MaxParallelJobs, hiChan)
			}
		}
	}

	// Catch any left overs.
	if opts.MaxParallelJobs > 1 {
		unsunk := len(opts.Hosts) % opts.MaxParallelJobs
		if unsunk == 0 {
			unsunk = opts.MaxParallelJobs
		}
		vinfon(opts, 2, "checking for remaining unsunk channel elements %v", unsunk)
		if unsunk > 0 {
			vinfon(opts, 2, "final unsunk = %v", unsunk)
			sink(unsunk, hiChan)
		}
	}
}

// Execute the command for all hosts.
func execCmd(hi hostinfo, opts options, hiChan chan hostinfo) {
	// lambda for handling goroutine errors
	cx := func(err error) bool {
		if err != nil {
			if len(hi.Output) > 0 && hi.Output[len(hi.Output)-1] != '\n' {
				hi.Output += "\n"
			}
			_, _, lineno, _ := runtime.Caller(1)
			hi.Output += fmt.Sprintf("ERROR:%v %v %v@%v - %v\n", lineno, hi.ID, hi.Username, hi.Host, err)
			hiChan <- hi
			return true
		}
		return false
	}

	vinfo(opts, "executing command on [%v] %v@%v", hi.ID, hi.Username, hi.Host)

	if len(opts.Command) == 0 {
		_, _, lineno, _ := runtime.Caller(0)
		cx(fmt.Errorf("ERROR:%v %v %v@%v - commmand cannot be zero length\n", lineno, hi.ID, hi.Username, hi.Host))
		return
	}

	// Create the connection.
	conn, err := tcpConnect(opts, hi)
	if cx(err) {
		return
	}
	session, err := conn.NewSession()
	if cx(err) {
		return
	}
	defer session.Close()

	// Collect the output from stdout and stderr.
	// The idea is to duplicate the shell IO redirection
	// comment 2>&1 where both streams are interleaved but
	// that doesn't work because each stream is handled
	// independently.
	stdoutPipe, err := session.StdoutPipe()
	if cx(err) {
		return
	}
	stderrPipe, err := session.StderrPipe()
	if cx(err) {
		return
	}
	outputReader := io.MultiReader(stdoutPipe, stderrPipe)
	outputScanner := bufio.NewScanner(outputReader)

	// Start the session.
	err = session.Start(opts.Command)
	if cx(err) {
		return
	}

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

	hi.Output = outputBuf
	hiChan <- hi
}

// Execute an interactive terminal.
// This only works for a single user.
func execTerm(opts options) {
	vinfo(opts, "creating interactive terminal")

	conn, err := tcpConnect(opts, opts.Hosts[0])
	check(err)
	session, err := conn.NewSession()
	check(err)
	defer session.Close()

	// Use the current terminal fds.
	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	err = session.RequestPty("xterm", 80, 40, modes)
	check(err)
	err = session.Shell()
	check(err)
	vinfo(opts, "remote shell started")
	err = session.Wait()
	check(err)
	vinfo(opts, "remote shell finished")
}

// tcpConnect
func tcpConnect(opts options, hi hostinfo) (*ssh.Client, error) {
	conn, err := ssh.Dial("tcp", hi.Host, hi.Config)
	for r := 0; r < opts.NumRetries; r++ {
		if err == nil {
			break
		}
		vinfon(opts, 2, "retry %v %v %v@%v", r+1, hi.ID, hi.Username, hi.Host)
		time.Sleep(time.Duration(200) * time.Millisecond)
		conn, err = ssh.Dial("tcp", hi.Host, hi.Config)
	}
	return conn, err
}

// Check for an error, if the error exists, repot it and exit.
func check(e error) {
	if e != nil {
		_, _, lineno, _ := runtime.Caller(1)
		log.Fatalf("ERROR:%v %v", lineno, e)
	}
}

// Print an error and exit.
func fatal(f string, args ...interface{}) {
	_, _, lineno, _ := runtime.Caller(1)
	f1 := fmt.Sprintf("ERROR:%04v %v", lineno, f)
	log.Fatalf(f1, args...)
}

// Print an info message in verbose mode.
func vinfo(opts options, f string, args ...interface{}) {
	if opts.Verbose > 0 {
		_, _, lineno, _ := runtime.Caller(1)
		f1 := fmt.Sprintf("INFO:%04v %v", lineno, f)
		log.Printf(f1, args...)
	}
}

// Print an info message for a specific level of verbosity.
func vinfon(opts options, level int, f string, args ...interface{}) {
	if opts.Verbose >= level {
		_, _, lineno, _ := runtime.Caller(1)
		f1 := fmt.Sprintf("INFO:%04v %v", lineno, f)
		log.Printf(f1, args...)
	}
}

// Info.
func info(f string, args ...interface{}) {
	_, _, lineno, _ := runtime.Caller(1)
	f1 := fmt.Sprintf("INFO:%04v %v", lineno, f)
	log.Printf(f1, args...)
}

// Warning.
func warning(f string, args ...interface{}) {
	_, _, lineno, _ := runtime.Caller(1)
	f1 := fmt.Sprintf("WARNING:%04v %v", lineno, f)
	log.Printf(f1, args...)
}

// Debug.
func debug(f string, args ...interface{}) {
	_, _, lineno, _ := runtime.Caller(1)
	f1 := fmt.Sprintf("DEBUG:%04v %v", lineno, f)
	log.Printf(f1, args...)
}
