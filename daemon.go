package siphon

/*

Siphon mode is largely implemented in siphon-cli.

This, lib siphon, knows about the Redirect messages, and the client reacts to those messages,
but a daemon itself that sends those messages is not here.

This is because siphon-cli's daemon actually execs out a new process to create a host,
which requires several assumptions that are somewhat beyond the reach of siphon-as-a-library.

*/
