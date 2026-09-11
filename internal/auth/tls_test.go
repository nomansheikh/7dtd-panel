package auth

import "crypto/tls"

// tlsState is a non-nil ConnectionState, which is all the code checks when
// deciding whether a request arrived over TLS.
var tlsState = tls.ConnectionState{}
