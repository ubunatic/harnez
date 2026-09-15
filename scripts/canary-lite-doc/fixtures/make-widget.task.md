Write a Makefile to ./Makefile for a small Go CLI project (binary name
'widget') that:
1) has a self-documenting help target as the default goal;
2) has a build target that compiles the binary;
3) has an install target that installs locally and system-wide with graceful
   degradation if sudo is unavailable;
4) has check and check-fast targets, plus a test alias;
5) uses the phony-sentinel convention instead of per-target .PHONY listing;
6) declares BINARY/CONFIG/TARGET/PREFIX-style variables appropriately.
