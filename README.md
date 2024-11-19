# FTL MySQL Auth Proxy

This project proxies a MySQL connection, intercepting the authentication messages to allow a client that does not
have credentials to connect to a MySQL server. This allows for a trust boundary model where a pod running user code
is never actually provided with database credentials.

This proxy should only ever be bound to localhost, as there is no authentication on the proxy itself.

## Implementation Notes

This proxy is based on the Golang MySQL drivers, and most of the files in this repo are copied from that repository. The
copied files should not be modified, and only modification should be made to the `ftl_*.go` files.

This will allow for easier updates to the MySQL driver in the future, as the proxy can be updated by copying the new
files. The mysql directory is a git submodule that should be updated to point to the new version.