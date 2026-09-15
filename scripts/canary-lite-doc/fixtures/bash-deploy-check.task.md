Write a bash script to ./deploy-check.sh that:
1) takes a required argument, a target directory to check;
2) verifies the directory exists, else prints an error and exits;
3) greps for the string TODO inside all .sh files in that directory using an
   awk-based per-file line-count summary (not just grep -c);
4) runs 'git status' scoped to that target directory without changing your
   own shell's working directory;
5) writes a temp file with the results and cleans it up automatically on exit;
6) prints success if zero TODOs found, else prints failure and exits non-zero.
