# sshx
Go demo program that shows how to the ssh package with keyboard-interactive, password and public-key

It's only goal is to provide some examples of how things work so that I
don't forget them. It is not suitable for production.

It demonstrates three types of authentication: password, keyboard-interactive
and public-key.

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

VERSION
        v0.1

```

## Auth Code
This is the authorization code.

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

	// This will not work if the following command fails.
	// $ ssh -o PreferredAuthentications=password localhost pwd
	// Permission denied (publickey,keyboard-interactive).
	config = &ssh.ClientConfig{
		User: opts.Username,
	}

	if opts.SSHPassword {
		config.Auth = append(config.Auth, ssh.Password(password))
	}

	if opts.SSHKeyboardInteractive {
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

	// Get the public key, if it is available.
	if opts.SSHPublicKey {
		if userData, err := user.Lookup(opts.Username); err == nil {
			idRsa := userData.HomeDir + "/.ssh/id_rsa"
			if _, err := os.Stat(idRsa); err == nil {
				if key, err := ioutil.ReadFile(idRsa); err == nil {
					if signer, err := ssh.ParsePrivateKey(key); err == nil {
						config.Auth = append(config.Auth, ssh.PublicKeys(signer))
					}
				}
			}
		}
	}

	return
}
```
