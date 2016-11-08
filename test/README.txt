Runs a simple test using local host.

You must put the current user password in passfile for it to work
or, alternatively, modify the test01.sh to prompt for the password
or add it directly into the script.

Here is how you run the tests:

   $ make
   ./test.sh
   ../sshx -j  0 -vv -P passfile +test-hosts.txt uname -a        15 real	0m7.558s user	0m0.048s sys	0m0.042s 
   ../sshx -j  1 -vv -P passfile +test-hosts.txt uname -a        15 real	0m7.252s user	0m0.056s sys	0m0.046s 
   ../sshx -j  2 -vv -P passfile +test-hosts.txt uname -a        15 real	0m3.655s user	0m0.053s sys	0m0.045s 
   ../sshx -j  3 -vv -P passfile +test-hosts.txt uname -a        15 real	0m3.008s user	0m0.063s sys	0m0.047s 
   ../sshx -j  4 -vv -P passfile +test-hosts.txt uname -a        15 real	0m2.567s user	0m0.059s sys	0m0.045s 
   ../sshx -j  5 -vv -P passfile +test-hosts.txt uname -a        15 real	0m1.924s user	0m0.059s sys	0m0.046s 
   ../sshx -j  6 -vv -P passfile +test-hosts.txt uname -a        15 real	0m2.350s user	0m0.059s sys	0m0.045s 
   ../sshx -j  7 -vv -P passfile +test-hosts.txt uname -a        15 real	0m1.846s user	0m0.056s sys	0m0.043s 
   ../sshx -j  8 -vv -P passfile +test-hosts.txt uname -a        15 real	0m2.186s user	0m0.059s sys	0m0.044s 
   ../sshx -j  9 -vv -P passfile +test-hosts.txt uname -a        15 real	0m2.328s user	0m0.059s sys	0m0.046s 
   ../sshx -j 10 -vv -P passfile +test-hosts.txt uname -a        15 real	0m3.611s user	0m0.060s sys	0m0.044s 
   ../sshx -j 11 -vv -P passfile +test-hosts.txt uname -a        15 real	0m3.077s user	0m0.060s sys	0m0.045s 
   ../sshx -j 12 -vv -P passfile +test-hosts.txt uname -a        15 real	0m2.642s user	0m0.058s sys	0m0.045s 
   ../sshx -j 13 -vv -P passfile +test-hosts.txt uname -a        15 real	0m2.699s user	0m0.058s sys	0m0.040s 
   ../sshx -j 14 -vv -P passfile +test-hosts.txt uname -a        15 real	0m1.630s user	0m0.059s sys	0m0.045s 
   ../sshx -j 15 -vv -P passfile +test-hosts.txt uname -a        15 real	0m3.428s user	0m0.058s sys	0m0.044s 
   ../sshx -j 16 -vv -P passfile +test-hosts.txt uname -a        15 real	0m2.476s user	0m0.059s sys	0m0.045s 

