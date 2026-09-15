Write a Makefile to ./Makefile for a Go service ('relayd') that is deployed
to a remote host, and that:
1) has a self-documenting help target as the default goal;
2) has a build target;
3) exposes the full set of deployment targets this project convention requires
   for any project that pushes a binary, configs, or schedules to a remote
   host — including a dry-run switch on the deploying target, a read-only
   liveness/health query target plus its conventional alias, and a target that
   pulls remote state snapshots back down;
4) uses the phony-sentinel convention instead of per-target .PHONY listing;
5) declares BINARY/PREFIX-style variables appropriately.
