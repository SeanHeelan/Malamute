# Malamute

Malamute is a fuzzing toolkit initially developed to aid with regression-test
based fuzzing of language interpreters. The ideas behind it were first presented
at Infiltrate 2014 as a part of my talk titled *Ghosts of Christmas Past:
Fuzzing Language Interpreters using Regression Tests*. This repository contains
the functionality described in the first half of that presentation, making it
suitable for fuzzing of interpreters with a command-line interface.

*As a word of warning, this was the first project I ever worked on in Go and
for that reason the code is 'interesting' in places ;)*

# Building

	git clone https://github.com/SeanHeelan/Malamute
	cd Malamute/bin/mfuzz
	go install

# Usage

I'll update this with real details soon. The above build commands install the
`mfuzz` command. Run it without any arguments for paramater information. Sample
configuration files can be found in the `ext` directory.

