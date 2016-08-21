/*
Program to jump to tags (e.g. generated by Exuberant Ctags) using the
Plan 9 plumber.

Usage:
	Ctag [ident]

Ident is the identifier looked up in the tags file. If it's not specified,
it uses the selected text in the current acme window or if nothing is
selected, the identifier located near the cursor.
*/
package main