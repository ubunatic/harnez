Write a bash script to ./version-probe.sh that:
1) defines a function `probe_version` taking a tool name, which captures the
   output of `"$tool" --version` into a local variable and returns non-zero if
   that command failed, without the capture hiding the command's exit status;
2) defines a function `report` that prints a success marker to stdout and a
   failure marker to stderr;
3) takes a required argument, a tool name, and errors out with a usage message
   if it is missing;
4) loads shared helpers from ./lib.sh if that file exists;
5) checks both that the tool is on PATH and that the probe succeeded before
   reporting success;
6) exits non-zero on any failure.
