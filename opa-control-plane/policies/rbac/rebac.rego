# TITLE: Role-Based Access Control
# DESCRIPTION: Basic RBAC policy for user authorization
# TAGS: rbac, security, authorization

package rbac

import rego.v1

allow if {
    user_has_role(input.user, input.required_role)
}

allow if {
    user_has_role(input.user, "admin")
}

user_has_role(user, role) if {
    role in user.roles
}

default allow := false