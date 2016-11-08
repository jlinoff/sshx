#!/bin/bash

if [ ! -f passfile ] ; then
    echo
    echo "ERROR: passfile does not exist! Cannot run the commands automatically."
    echo "       You can fix this by creating the passfile with the local user password"
    echo "       or by changing this script ($0), to prompt for the password and then"
    echo "       passing it on the command line."
    echo
    exit 1
fi

for (( i = 0 ; i < 17 ; i++ )) ; do
    printf "../sshx -j %2d -vv -P passfile +test-hosts.txt uname -a  " $i
    #../sshx -j $i -vv -P passfile +test-hosts.txt uname -a 2>&1 | grep '# Job ' | wc -l
    ( time ( ../sshx -j $i -vv -P passfile +test-hosts.txt uname -a 2>&1 | grep '# Job ' | wc -l ) 2>&1 ) | \
        egrep '^real|^user|^sys|  [0-9]' | \
        tr '\n' ' '
    echo
done
