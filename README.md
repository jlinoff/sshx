# sshx
Go demo program that shows how to the ssh package with keyboard-interactive, password and public-key

It's only goal is to provide some examples of how things work so that I
don't forget them. It is not suitable for production.

It demonstrates three types of authentication: password, keyboard-interactive
and public-key.

It also demonstrates how to set the HostKeyAlgorithms field in the ClientConfig
and where to find the legal values (`ssh -Q key`).

And, finally, it demonstrates how to implement a remote interactive terminal if
no command is specified.

Any comments or suggestions to improve it or fix mistakes are greatly appreciated.

## Download and Build it

If you have make installed do this.
```bash
$ git clone https://github.com/jlinoff/sshx.git
$ cd sshx
$ make
```

If you don't.
```bash
$ git clone https://github.com/jlinoff/sshx.git
$ cd sshx
$ GOPATH=$(pwd) go get golang.org/x/crypto/ssh
$ GOPATH=$(pwd) go build -o $@ main.go getpassword.go
```

## Simple example
Here is how you run a simple command.
```bash
$ ./sshx host1 pwd
```

## Help
Here is the program help.
```bash
$ ./sshx -h

USAGE
    sshx [OPTIONS] [<username>@]<host>[:<port>] <cmd>

DESCRIPTION
    Demonstration program that shows how to use the go ssh package to execute
    a command on a remote host using the SSH protocol.

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
    $ sshx -h

    # Example 2. Run a command using all of the authorization modes with
    #            the default user and port 22.
    $ sshx host1 pwd

    # Example 3: Explicitly specify a user and a port.
    $ sshx user@host1:22 pwd

    # Example 4: Only use password mode. This is equivalent to running
    #            sh -o PreferredAuthentications=password host1 pwd
    $ sshx -a password host1 pwd

    # Example 5: Only use keyboard-interactive mode.
    $ sshx -a keyboard-interactive host1 pwd

    # Example 6: Only use password and keyboard-interactive mode.
    $ sshx -a password,keyboard-interactive host1 pwd

    # Example 7: Specify a single host-key-algorithm.
    $ sshx -A id_rsa host1 uptime

    # Example 8: Start a remote shell.
    $ sshx host2

    # Example 9: Start a remote shell in verbose mode.
    $ sshx -v host2

VERSION
    v0.4

```

## Auth Code
This is the authorization configuration code.

```go
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
```
