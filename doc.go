// Package jano provides HTTP routing, route groups, middleware and error-returning
// request handlers without external dependencies.
//
// Jano implements http.Handler. Routes use whole-segment parameters such as
// /people/{id} and terminal catch-alls such as /files/{path...}. Parameters are
// available through Param, Request.PathValue, and legacy string context keys.
//
// Configuration changes are synchronized and served through immutable snapshots.
// Handlers, middleware and injected services remain responsible for their own
// shared state. Use the request context for database operations and cancellation.
package jano
