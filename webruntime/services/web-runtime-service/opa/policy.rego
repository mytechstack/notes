package webruntime.authz

# RFC goal G7: "Access control is embedded, not networked" — this policy is
# evaluated in-process via the OPA Go SDK (rego package), no extra hop.
#
# input: {
#   "roles":    [<roles from the caller's JWT>],
#   "required": [<roles from manifest.shell.auth.roles>]
# }

default allow = false

# No roles required by the manifest -> open access.
allow {
	count(input.required) == 0
}

# Otherwise allow if the caller has at least one of the required roles.
allow {
	some role
	input.roles[_] == input.required[role]
}
