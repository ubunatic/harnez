Write a bash script to ./log-summary.sh that:
1) takes a required argument, a log file path;
2) builds a summary by chaining at least three commands in a single long
   pipeline (filter, extract a field, count/sort) assigned to a variable;
3) only prints the summary when the log file both exists and is non-empty,
   using a single multi-condition check rather than nested checks;
4) fetches a remote status endpoint whose response time is unpredictable, and
   does not let that call hang indefinitely;
5) iterates over the resulting summary lines and prints each one, aborting the
   loop with an error if a line is malformed;
6) prints an ERROR line on stderr and exits non-zero on any failure.
