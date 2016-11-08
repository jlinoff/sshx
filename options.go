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
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

type hostinfo struct {
	Username string
	Password string // per host password
	Host     string // includes the port (e.g. localhost:22)
	Config   *ssh.ClientConfig
	HostFile string
	ID       int
	Output   string // filled in when the job is run
}

type options struct {
	Hosts                  []hostinfo
	Password               string // default password
	Command                string
	SSHKeyboardInteractive bool
	SSHPassword            bool
	SSHPublicKey           bool
	HostKeyAlgorithms      []string
	Verbose                int
	JobHeader              bool
	MaxParallelJobs        int
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

	// lambda to get a range in an interval
	nextArgInt := func(idx *int, o string, min int, max int) (arg int) {
		a := nextArg(idx, o)
		arg = 0
		if v, e := strconv.Atoi(a); e == nil {
			if v < min {
				log.Fatalf("ERROR: '%v' too small, minimum accepted value is %v", o, min)
			} else if v > max {
				log.Fatalf("ERROR: '%v' too large, maximum value accepted is %v", o, max)
			}
			arg = v
		} else {
			log.Fatalf("ERROR: '%v' expected a number in the range [%v..%v]", o, min, max)
		}
		return
	}

	opts.JobHeader = true
	opts.MaxParallelJobs = -1
	auth := "keyboard-interactive,password,public-key"
	i := 1
	foundHosts := false
	for ; i < len(os.Args) && foundHosts == false; i++ {
		opt := os.Args[i]
		switch opt {
		case "-a", "--auth":
			auth = nextArg(&i, opt)
		case "-A", "--algorithms":
			// Add a host key algorithm from the list provided by:
			// ssh -Q key
			alg := nextArg(&i, opt)
			algs := strings.Split(alg, ",")
			for _, a := range algs {
				a = strings.TrimSpace(a)
				opts.HostKeyAlgorithms = append(opts.HostKeyAlgorithms, a)
			}
		case "-h", "--help":
			help()
		case "-j", "--max-jobs":
			opts.MaxParallelJobs = nextArgInt(&i, opt, 0, 1000000)
		case "-n", "--no-job-header":
			opts.JobHeader = false
		case "-p", "--password":
			if len(opts.Password) != 0 {
				warning("overwriting previous password setting")
			}
			opts.Password = nextArg(&i, opt)
		case "-P", "--password-file":
			if len(opts.Password) != 0 {
				warning("overwriting previous password setting")
			}
			pf := nextArg(&i, opt)
			opts.Password = readPasswordFromFile(pf)
		case "-v", "--verbose":
			opts.Verbose++
		case "-vv", "-vvv":
			opts.Verbose += len(opt) - 1
		case "-V", "--version":
			fmt.Printf("%v v%v\n", getProgramName(), version)
			os.Exit(0)
		default:
			if strings.HasPrefix(opt, "-") {
				log.Fatalf("ERROR: unrecognized option '%v'", opt)
			}
			m := map[string]bool{}
			opts.Hosts = parseHostString(opt, m)
			foundHosts = true
		}
	}

	// The rest of the command line is the command to execute.
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

	// Post pass to update the passwords for each host to avoid having to check
	// it later.{
	if len(opts.Password) > 0 {
		for _, hi := range opts.Hosts {
			if len(hi.Password) == 0 {
				hi.Password = opts.Password
			}
		}
	}

	// Assume that we can have a channel per host/job unless told
	// otherwise.
	j := len(opts.Hosts)
	if opts.MaxParallelJobs < 0 {
		opts.MaxParallelJobs = j
	} else if j < opts.MaxParallelJobs {
		opts.MaxParallelJobs = j
	}

	// Output some information in verbose mode.
	vinfo(opts, "Cmd      = %v", opts.Command)
	vinfo(opts, "Max Jobs = %v", opts.MaxParallelJobs)
	vinfo(opts, "Auth     = %v", auth)
	vinfo(opts, "Hosts    = %v", len(opts.Hosts))
	for i, hi := range opts.Hosts {
		vinfo(opts, "           [%3d] %v %v %v %v", i+1, hi.ID, hi.Host, hi.Username, hi.HostFile)
	}

	return
}

// readPasswordFromFile reads the password from a file.
// Only read the first line.
// Can't used '#' for comments because that may start a
// password.
// Blank lines are ignored.
func readPasswordFromFile(fn string) (password string) {
	ifp, err := os.Open(fn)
	check(err)
	defer ifp.Close()

	scanner := bufio.NewScanner(ifp)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) > 0 {
			password = line
			break
		}
	}
	return
}

// parseHostString parses the host string, multiple hosts
// can be specified in a comma separated list with no
// whitespace.
//
// Highlevel syntax: <host>[,<host>]*
// Each host specification has the following syntax:
//
//   [<username>[:<password>]@]<hostname>[:<port>]
//
// It is okay for the password to contain an embedded
// "@", only the last one is picked up.
// Cannot handle commas in tbe password, make sure that is documented.
//
// Here are some examples:
//   host1
//   me@host1
//   me:@dumbpassword@@host1
//   host:22
func parseHostString(data string, m map[string]bool) (hosts []hostinfo) {
	hostSpecs := strings.Split(data, ",")
	for _, hostSpec := range hostSpecs {
		// Parse each user.
		if strings.HasPrefix(hostSpec, "+") {
			// This is a file of the form +<file>.
			fn := hostSpec[1:]
			his := parseHostFile(fn, m)
			for _, hi := range his {
				hi.ID = len(hosts) + 1
				hosts = append(hosts, hi)
			}
		} else {
			// This a host specification of the form:
			//    [<username>[:<password>]@]<host>[:<port>]
			pos := strings.LastIndex(hostSpec, "@")
			user := ""
			pass := ""
			host := ""
			if pos >= 0 {
				// A "@" is present.
				left := hostSpec[:pos] // username or username:password
				host = hostSpec[pos+1:]
				flds := strings.SplitN(left, ":", 2)
				if len(flds) == 2 {
					// me:password@host
					user = flds[0]
					pass = flds[1]
				} else {
					// me@host
					user = flds[0]
				}
			} else {
				// @ is not present.
				user = strings.TrimSpace(os.Getenv("LOGNAME"))
				host = hostSpec
			}

			if strings.Contains(host, ":") == false {
				host += ":22"
			}

			hi := hostinfo{
				Host:     host,
				Username: user,
				Password: pass,
				ID:       len(hosts) + 1,
			}
			hosts = append(hosts, hi)
		}
	}
	return
}

// parseHostFile parses a host file.
func parseHostFile(fn string, m map[string]bool) (hosts []hostinfo) {
	// Catch nested references to the same file to avoid infinite recursion.
	afn, _ := filepath.Abs(fn)
	if _, found := m[afn]; found == true {
		fatal("nested reference found to file '%v'", afn)
	}
	m[afn] = true

	// Read the file line by line and parse it.
	// Ignore lines that are blank or start with '#'.
	ifp, err := os.Open(fn)
	check(err)
	defer ifp.Close()

	scanner := bufio.NewScanner(ifp)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		his := parseHostString(line, m)
		for _, hi := range his {
			hi.HostFile = fn
			hosts = append(hosts, hi)
		}
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
    %[1]v [OPTIONS] <host-spec>[,<host-spec>] <cmd>

    Where <host-spec>:

        [<username>[:<password>]@]<host>[:<port>]
          ^           ^            ^       ^
          |           |            |       +-- optional, port, defaults to 22
          |           |            +---------- required, host name or IP addr
          |           +----------------------- optional, password - commas not
          |                                              allowed, will use -p
          |                                              if not specified
          +----------------------------------- optional, username, def LOGNAME

        +<host-file>
          ^
          +-------- file that contains host specifications or references to
                    other hosts

DESCRIPTION
    Demonstration program that shows how to use the go ssh package to execute
    a command on one or more remote hosts using the SSH protocol.

    It's only goal is to provide some examples of how things work so that I
    don't forget them. It is not suitable for production.

    It supports three types of authentication: password, keyboard-interactive
    and public-key.

    It demonstrates how to set the HostKeyAlgorithms field in the
    ssh.ClientConfig and where to find the legal values (ssh -Q key).

    It also demonstrates how to start a remote interactive shell when no
    command is specified.

    If the username is not specified, the username of the current user is used.

    If the port is not specified, port 22 is used.

    The host files referenced in the USAGE section are text files with one host
    or host-file reference per line. Lines whose first non-whitespace character
    are '#' are ignored. Blank lines are ignored.

OPTIONS
    -a MODES, --auth MODES
                       Explicitly specify the authorization modes in a comma
                       separated list. Three modes are recognized.
                           1. keyboard-interactive
                           2. password
                           3. public-key
                       It is case-insenstive.
                       By default all modes are enabled.

    -A ALGORITHMS, --algorithms ALGORITHMS
                       Explicitly specify the the host key algorithms that you
                       want to use in a comma separated list.
                       Here is an example.
                           -A id_rsa,id_dsa
                       To see the host key algorithms available on your system
                       run "ssh -Q key".

    -h, --help         This help message.

    -j NUM, --max-jobs NUM
                       The maximum number of jobs that can be run concurrently.
                       This option basically describes the width of the channel.
                       The default is the number of hosts/jobs.

    -n, --no-job-header
                       Turns off the job header for each host. The job header
                       is printed to make it easier to differentiate between
                       the output from different hosts.

    -p STRING, --password STRING
                       Define the password for password and keyboard-interactive
                       authorization operations.
                       DO NOT use this on the command line. It may show up
                       in your history file. Go ahead and use it in a 0755
                       shell script.
                       If the password is not specified, you will be prompted.

    -P FILE, -password-file FILE
                       Read the password from a password file.

    -v, --verbose      Increase the level of verbosity.
                       You can use -vv as shorthand to specify -v -v.

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

    # Example 7: Specify a single host-key-algorithm.
    $ %[1]v -A id_rsa host1 uptime

    # Example 8: Start a remote shell.
    $ %[1]v host2

    # Example 9: Start a remote shell in verbose mode.
    $ %[1]v -v host2

    # Example 10: Run a command on multiple hosts.
    $ %[1]v host1,host2,host3 /bin/bash -c "echo && hostname && date && uptime"

    # Example 11: Run a command on multiple hosts using a host file.
    #             The equals sign designates a file rather than a host.
    #             Can have as many files and hosts as you like, intermixed.
    $ cat >hosts.txt <<EOF
    ### My hosts file.
    host1
    host2:22
    me@host3:22

    # include another file
    +other-hosts.txt
    EOF
    $ %[1]v +hosts.txt,host4 uname -r

    # Example 12: Run a command on 20 hosts, limit concurrency to 10.
    $ %[1]v -j 10 +hosts-20.txt uptime

VERSION
    v%[2]v
`
	f = "\n" + strings.TrimSpace(f) + "\n\n"
	fmt.Printf(f, getProgramName(), version)
	os.Exit(0)
}
