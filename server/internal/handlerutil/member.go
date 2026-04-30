package handlerutil

import db "github.com/agentra-ai/agentra/server/pkg/db/generated"

func RoleAllowed(role string, roles ...string) bool {
	for _, candidate := range roles {
		if role == candidate {
			return true
		}
	}
	return false
}

func CountOwners(members []db.Member) int {
	n := 0
	for _, m := range members {
		if m.Role == "owner" {
			n++
		}
	}
	return n
}