# TITLE: Data Access Control
# DESCRIPTION: Policy for controlling data access based on ownership

package data.access

import rego.v1

allow if {
    input.action == "read"
    input.resource.owner == input.user.id
}

allow if {
    "admin" in input.user.roles
}

default allow := false