// Package nextadapter: transport.go previously contained parserTransport,
// which bridged v2-alpha RequestContext to the internal endpoint. This
// is no longer needed because routing now provides routing.Transport
// directly. The transportAdapter in wrap.go replaces it.
package nextadapter
