
# go-telnet
[![Build Status](https://travis-ci.org/morganhein/go-telnet.svg?branch=master)](https://travis-ci.org/morganhein/go-telnet)


A net.Conn compatible implementation with telnet support.

This is a drop-in replacement for net.Dial that handles telnet negotiaton and other out-of-band messages transparently.

It currently refuses and/or disables all options in a sane manner, except for binary transmission. It disables the Go-Ahead and and ECHO options as well.

Further work needs to be done to implement other telnet options. This is planned, however I have little motivation to do so at the moment.

