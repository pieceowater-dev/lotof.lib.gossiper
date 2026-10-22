// Package mcpkit is the reusable half of a Model Context Protocol endpoint for AI clients (Claude, ChatGPT, ...):
// everything that is the same in every lota product, so a service only writes its own tools.
//
// What it gives a service:
//   - Handler: an http.Handler (streamable HTTP, stateless) over the official go-sdk, with the request body capped,
//     authentication by an access key ("Authorization: Bearer lpt_...") resolved through the Resolver interface,
//     a brake on guessing keys per address, a 401 that carries resource_metadata so a client can start its own
//     sign-in, and one log line per tool call (who, which tool, outcome, never the arguments).
//   - Principal: who the key belongs to (namespace, user, optional site binding, scopes) with Need and Site checks.
//   - AddTool: registers a typed tool behind a permission class (Read, Write, Act), so a tool cannot forget its check.
//   - GRPCError: turns a gRPC status into the plain message a model can act on.
//   - package mcpkit/oauth: the optional sign-in server for AI clients (client registration, authorize -> consent page,
//     token with PKCE, sealed one-time codes) that hands out ordinary access keys through a KeyIssuer.
//
// Changes to this package are additive: services pin a version of the library and nothing here is required by
// the rest of gossiper.
package mcpkit
