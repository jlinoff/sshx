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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type options struct {
	Username               string
	Password               string
	Command                string
	Host                   string
	SSHKeyboardInteractive bool
	SSHPassword            bool
	SSHPublicKey           bool
	Verbose                int
}

// getopts - gets the command line options and populations the options
// structure for use in main.
func getopts() (opts options) {
	// lambda to get the next argument on the command line.
	nextArg := func(idx *int, o string) (arg string) {
		*idx++
		if *idx < len(os.Args) {
			arg = os.Args[*idx]
		} else {
			log.Fatalf("ERROR: missing argumnent for option '%s'", o)
		}
		return
	}

	auth := "keyboard-interactive,password,public-key"
	i := 1
	for ; i < len(os.Args) && len(opts.Host) == 0; i++ {
		opt := os.Args[i]
		switch opt {
		case "-a", "--auth":
			auth = nextArg(&i, opt)
		case "-h", "--help":
			help()
		case "-p", "--password":
			opts.Password = nextArg(&i, opt)
		case "-v", "--verbose":
			opts.Verbose++
		case "-V", "--version":
			fmt.Printf("%v v%v\n", getProgramName(), version)
			os.Exit(0)
		default:
			if strings.HasPrefix(opt, "-") {
				log.Fatalf("ERROR: unrecognized option '%v'", opt)
			}
			opts.Host = opt
			if strings.Contains(opts.Host, ":") == false {
				opts.Host += ":22"
			}
			if strings.Contains(opts.Host, "@") == false {
				opts.Username = strings.TrimSpace(os.Getenv("LOGNAME"))
			} else {
				flds := strings.SplitN(opts.Host, "@", 2)
				opts.Host = flds[0]
				opts.Username = flds[1]
			}
		}
	}
	for ; i < len(os.Args); i++ {
		if len(opts.Command) > 0 {
			opts.Command += " "
		}
		opts.Command += quote(os.Args[i])
	}

	// Parse auth.
	ms := strings.Split(auth, ",")
	for _, m := range ms {
		switch strings.ToLower(strings.TrimSpace(m)) {
		case "keyboard-interactive":
			opts.SSHKeyboardInteractive = true
		case "password":
			opts.SSHPassword = true
		case "public-key":
			opts.SSHPublicKey = true
		default:
			log.Fatalf("ERROR: unrecognized auth mode '%v', valid modes: keyboard-interactive, password, public-key", m)
		}
	}

	if opts.Verbose > 0 {
		fmt.Println("")
		fmt.Printf("Cmd  : %v\n", opts.Command)
		fmt.Printf("Host : %v\n", opts.Host)
		fmt.Printf("User : %v\n", opts.Username)
		fmt.Printf("Auth : %v\n", auth)
		fmt.Println("")
	}

	return
}

// Get the program name.
func getProgramName() string {
	x, _ := filepath.Abs(os.Args[0])
	return filepath.Base(x)
}

// Quote an individual token.
// Very simple, not suitable for production.
func quote(token string) string {
	q := false
	r := ""
	for _, c := range token {
		switch c {
		case '"':
			q = true
			r += "\""
		case ' ', '\t':
			q = true
		}
		r += string(c)
	}
	if q {
		r = "\"" + r + "\""
	}
	return r
}

// help
func help() {
	f := `
USAGE
    %[1]v [OPTIONS] [<username>@]<host>[:<port>] <cmd>

DESCRIPTION
    Demonstration program that shows how to use the go ssh package to execute
    a command on a remote host using the SSH protocol.

    It's only goal is to provide some examples of how things work so that I
    don't forget them. It is not suitable for production.

    It supports three types of authentication: password, keyboard-interactive
    and public-key.

    If the username is not specified, the username of the current user is used.

    If the port is not specified, port 22 is used.

OPTIONS
    -a MODES, --auth MODES
                       Explicitly specify the authorization modes in a comma
                       separated list. Three modes are recognized.
                           1. keyboard-interactive
                           2. password
                           3. public-key
                       It is case-insenstive.
                       By default all modes are enabled.

    -h, --help         This help message.

    -p STRING, --password STRING
                       Define the password for password and keyboard-interactive
                       authorization operations.
                       DO NOT use this on the command line. It may show up
                       in your history file. Go ahead and use it in a 0755
                       shell script.
                       If the password is not specified, you will be prompted.

    -v, --verbose      Increase the level of verbosity.

    -V, --version      Print the program version and exit.

EXAMPLES
    # Example 1. help
    $ %[1]v -h

    # Example 2. Run a command using all of the authorization modes with
    #            the default user and port 22.
    $ %[1]v host1 pwd

    # Example 3: Explicitly specify a user and a port.
    $ %[1]v user@host1:22 pwd

    # Example 4: Only use password mode. This is equivalent to running
    #            sh -o PreferredAuthentications=password host1 pwd
    $ %[1]v -a password host1 pwd

    # Example 5: Only use keyboard-interactive mode.
    $ %[1]v -a keyboard-interactive host1 pwd

    # Example 6: Only use password and keyboard-interactive mode.
    $ %[1]v -a password,keyboard-interactive host1 pwd

VERSION
        v%[2]v
`
	f = "\n" + strings.TrimSpace(f) + "\n\n"
	fmt.Printf(f, getProgramName(), version)
	os.Exit(0)
}
