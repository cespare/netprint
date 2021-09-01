# netprint

A pretty-printer for network requests.

## Notes

netprint only handles a single connection at a time, to ensure the output is
easy to understand.

## To Do

* Nice colorized printing
* `-v`, verbose mode -- when serving HTTP, print everything in the request
  (headers, etc).
* Pretty-print json (if valid)
* `-x` for hex dumping the result instead of printing directly. Also hex-dump
  automatically if the result is a binary mime type.
