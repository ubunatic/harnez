Write a Makefile to ./Makefile for a Go CLI ('sprocket') that:
1) has exactly one .PHONY line in the whole file, no matter how many targets
   exist, and still treats every target as phony;
2) distinguishes targets that a tooling generator keeps reconciled from targets
   a human wrote once and the generator must never overwrite, using the
   convention's markers;
3) has a help target that discovers its own target list and descriptions by
   scraping the Makefile itself, with colored target names;
4) has build, check, check-fast, test and clean targets, all discoverable from
   help;
5) makes check run static analysis before tests.
