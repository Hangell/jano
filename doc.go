// Package jano provides HTTP routing with method-specific handlers and middleware.
//
// Routes use whole-segment parameters such as /people/{id}. Parameter values are
// available through the request context using their string names. Configure a
// Jano instance before serving requests; concurrent configuration is unsupported.
package jano
